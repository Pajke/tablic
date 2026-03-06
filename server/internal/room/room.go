package room

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"tablic/server/internal/game"
	"tablic/server/internal/protocol"
	"tablic/server/internal/storage"
)

const writeChannelSize = 32

// playerConn holds a WebSocket connection and write channel for one player.
type playerConn struct {
	playerID string
	conn     *websocket.Conn
	writeCh  chan []byte
	done     chan struct{}
}

// Room is an authoritative game room.
type Room struct {
	mu         sync.Mutex
	id         string
	maxPlayers int
	state      *game.GameState
	conns      map[string]*playerConn // playerID → conn
	tokens     map[string]string      // playerID → reconnect token
	names      map[string]string      // playerID → display name
	seatOrder  []string               // playerIDs in join order → deterministic seat indices
	storage    *storage.Storage       // may be nil

	// pendingCapture holds options waiting for CHOOSE_CAPTURE from the current player.
	pendingCapture []game.CaptureOption
	pendingCardID  string

	// turnTimer fires if the current player stays disconnected for too long.
	turnTimer *time.Timer
}

func newRoom(id string, maxPlayers int, st *storage.Storage) *Room {
	return &Room{
		id:         id,
		maxPlayers: maxPlayers,
		conns:      make(map[string]*playerConn),
		tokens:     make(map[string]string),
		names:      make(map[string]string),
		storage:    st,
	}
}

func (r *Room) ID() string { return r.id }

// Join adds a player to the room before the game starts.
// Returns playerID, reconnect token, seat index, or error if full.
func (r *Room) Join(name string) (string, string, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != nil {
		return "", "", 0, errors.New("game already started — use reconnect")
	}
	if len(r.conns) >= r.maxPlayers {
		return "", "", 0, errors.New("room is full")
	}

	playerID := generateID()
	token := generateID()
	seatIndex := len(r.conns)

	r.conns[playerID] = &playerConn{
		playerID: playerID,
		writeCh:  make(chan []byte, writeChannelSize),
		done:     make(chan struct{}),
	}
	r.tokens[playerID] = token
	r.names[playerID] = name
	r.seatOrder = append(r.seatOrder, playerID)

	return playerID, token, seatIndex, nil
}

// Reconnect finds a player by reconnect token and re-attaches them.
// Returns playerID and seat index, or error if token invalid.
func (r *Room) Reconnect(token string) (string, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, t := range r.tokens {
		if t == token {
			if r.state == nil {
				return "", 0, errors.New("game not started yet")
			}
			for _, p := range r.state.Players {
				if p.ID == id {
					return id, p.SeatIndex, nil
				}
			}
		}
	}
	return "", 0, errors.New("invalid reconnect token")
}

// AttachConn assigns a live WebSocket to a joined player and starts their write goroutine.
func (r *Room) AttachConn(playerID string, conn *websocket.Conn) error {
	r.mu.Lock()
	pc, ok := r.conns[playerID]
	if !ok {
		r.mu.Unlock()
		return errors.New("player not in room")
	}
	// Replace done channel so the new write goroutine has a fresh signal.
	pc.done = make(chan struct{})
	pc.conn = conn
	// If this player was the current player, cancel any pending auto-skip timer.
	if r.state != nil && r.state.CurrentPlayer().ID == playerID {
		r.cancelTurnTimer()
	}
	r.mu.Unlock()

	go r.writeLoop(pc)
	return nil
}

// PlayerCount returns the current number of joined players.
func (r *Room) PlayerCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.conns)
}

// IsFull returns true when the room has reached maxPlayers.
func (r *Room) IsFull() bool {
	return r.PlayerCount() >= r.maxPlayers
}

// StartGame initialises the GameState and deals the first hand. Call once the room is full.
func (r *Room) StartGame() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	players := make([]game.Player, 0, len(r.seatOrder))
	for _, id := range r.seatOrder {
		players = append(players, game.Player{
			ID:   id,
			Name: r.names[id],
		})
	}

	r.state = game.NewGame(players, r.id)
	if err := r.state.DealNextHand(); err != nil {
		return err
	}
	return nil
}

// GameState returns the current game state (for sending on reconnect). Caller holds no lock.
func (r *Room) ClientState() game.ClientGameState {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		return game.ClientGameState{}
	}
	return r.state.ToClientState()
}

// PlayerHand returns a player's current hand (for sending on reconnect).
func (r *Room) PlayerHand(playerID string) []game.Card {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		return nil
	}
	for _, p := range r.state.Players {
		if p.ID == playerID {
			return p.Hand
		}
	}
	return nil
}

// HandleMessage processes a parsed client message. Called from the read goroutine.
func (r *Room) HandleMessage(playerID string, msgType string, msg any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == nil {
		return
	}

	switch msgType {
	case "PLAY_CARD":
		m := msg.(protocol.PlayCardMsg)
		r.handlePlayCard(playerID, m.CardID)
	case "CHOOSE_CAPTURE":
		m := msg.(protocol.ChooseCaptureMsg)
		r.handleChooseCapture(playerID, m.OptionIndex)
	case "PING":
		r.sendTo(playerID, protocol.MustMarshal(protocol.PongMsg{Type: "PONG"}))
	}
}

// handlePlayCard processes a PLAY_CARD (mu held).
func (r *Room) handlePlayCard(playerID, cardID string) {
	r.cancelTurnTimer()
	cur := r.state.CurrentPlayer()
	if cur.ID != playerID {
		log.Printf("[room %s] PLAY_CARD rejected: not %s's turn (current=%s)", r.id, playerID, cur.ID)
		r.sendError(playerID, "NOT_YOUR_TURN", "it is not your turn")
		return
	}
	if r.pendingCapture != nil {
		log.Printf("[room %s] PLAY_CARD rejected: capture pending for card %s", r.id, r.pendingCardID)
		r.sendError(playerID, "CAPTURE_PENDING", "choose a capture option first")
		return
	}

	var playedCard *game.Card
	for i := range cur.Hand {
		if cur.Hand[i].ID == cardID {
			playedCard = &cur.Hand[i]
			break
		}
	}
	if playedCard == nil {
		log.Printf("[room %s] PLAY_CARD rejected: card %s not in hand %s", r.id, cardID, formatCards(cur.Hand))
		r.sendError(playerID, "CARD_NOT_IN_HAND", "card not in hand: "+cardID)
		return
	}

	log.Printf("[room %s] PLAY_CARD: player=%s card=%s table=[%s]",
		r.id, r.names[playerID], playedCard.ID, formatCards(r.state.TableCards))

	result := game.ComputeCaptures(*playedCard, r.state.TableCards)
	log.Printf("[room %s] capture result: %d option(s) — %s", r.id, len(result.Options), formatOptions(result.Options))

	r.broadcast(protocol.MustMarshal(protocol.CardPlayedMsg{
		Type:     "CARD_PLAYED",
		PlayerID: playerID,
		Card:     *playedCard,
	}))

	switch len(result.Options) {
	case 0:
		log.Printf("[room %s] → discard %s", r.id, playedCard.ID)
		discardedCard := *playedCard // copy before ApplyDiscard shifts the slice
		if err := r.state.ApplyDiscard(playerID, cardID); err != nil {
			r.sendError(playerID, "INVALID_MOVE", err.Error())
			return
		}
		r.broadcast(protocol.MustMarshal(protocol.CardDiscardedMsg{
			Type: "CARD_DISCARDED",
			Card: discardedCard,
		}))
		r.advanceTurnOrDeal()

	case 1:
		captured := flattenGroups(result.Options[0].Groups)
		log.Printf("[room %s] → auto-capture [%s]", r.id, formatCards(captured))
		wasTabla, err := r.state.ApplyCapture(playerID, cardID, result.Options[0])
		if err != nil {
			r.sendError(playerID, "INVALID_MOVE", err.Error())
			return
		}
		if wasTabla {
			log.Printf("[room %s] → TABLA!", r.id)
		}
		r.broadcast(protocol.MustMarshal(protocol.CaptureMadeMsg{
			Type:          "CAPTURE_MADE",
			PlayerID:      playerID,
			CapturedCards: captured,
			WasTabla:      wasTabla,
		}))
		r.advanceTurnOrDeal()

	default:
		log.Printf("[room %s] → %d options sent to player (needs choice)", r.id, len(result.Options))
		r.pendingCapture = result.Options
		r.pendingCardID = cardID
		r.sendTo(playerID, protocol.MustMarshal(protocol.CaptureOptionsMsg{
			Type:    "CAPTURE_OPTIONS",
			Options: result.Options,
		}))
	}
}

// handleChooseCapture processes a CHOOSE_CAPTURE (mu held).
func (r *Room) handleChooseCapture(playerID string, optionIndex int) {
	cur := r.state.CurrentPlayer()
	if cur.ID != playerID {
		r.sendError(playerID, "NOT_YOUR_TURN", "it is not your turn")
		return
	}
	if r.pendingCapture == nil {
		r.sendError(playerID, "NO_PENDING_CAPTURE", "no capture choice is pending")
		return
	}
	if optionIndex < 0 || optionIndex >= len(r.pendingCapture) {
		r.sendError(playerID, "INVALID_OPTION", "option index out of range")
		return
	}

	opt := r.pendingCapture[optionIndex]
	r.pendingCapture = nil
	captured := flattenGroups(opt.Groups)

	log.Printf("[room %s] CHOOSE_CAPTURE: player=%s chose option %d → [%s]",
		r.id, r.names[playerID], optionIndex, formatCards(captured))

	wasTabla, err := r.state.ApplyCapture(playerID, r.pendingCardID, opt)
	if err != nil {
		r.sendError(playerID, "INVALID_MOVE", err.Error())
		return
	}
	if wasTabla {
		log.Printf("[room %s] → TABLA!", r.id)
	}
	r.broadcast(protocol.MustMarshal(protocol.CaptureMadeMsg{
		Type:          "CAPTURE_MADE",
		PlayerID:      playerID,
		CapturedCards: captured,
		WasTabla:      wasTabla,
	}))
	r.advanceTurnOrDeal()
}

// advanceTurnOrDeal advances the turn, deals if needed, or ends the round (mu held).
func (r *Room) advanceTurnOrDeal() {
	r.state.AdvanceTurn()

	if !r.state.AllHandsEmpty() {
		r.logState("turn")
		r.broadcast(protocol.MustMarshal(protocol.TurnStartMsg{
			Type:        "TURN_START",
			PlayerIndex: r.state.CurrentPlayerIndex,
		}))
		r.maybeStartTurnTimer()
		return
	}

	if !r.state.DeckEmpty() {
		if err := r.state.DealNextHand(); err != nil {
			log.Printf("[room %s] deal error: %v", r.id, err)
			return
		}
		log.Printf("[room %s] NEW DEAL (deal #%d)", r.id, r.state.DealNumber)
		r.logState("new deal")
		for _, p := range r.state.Players {
			r.sendTo(p.ID, protocol.MustMarshal(protocol.HandDealtMsg{
				Type:  "HAND_DEALT",
				Cards: p.Hand,
			}))
		}
		r.broadcast(protocol.MustMarshal(protocol.TurnStartMsg{
			Type:        "TURN_START",
			PlayerIndex: r.state.CurrentPlayerIndex,
		}))
		r.maybeStartTurnTimer()
		return
	}

	// End of round
	r.state.ApplyLastHandRule()
	scores := r.state.ComputeRoundScores()
	log.Printf("[room %s] ── ROUND %d END ─────────────────────────────────────", r.id, r.state.RoundNumber)
	for _, sc := range scores {
		log.Printf("[room %s]   %-12s  cards: %2d  spil: %d  tabla: %d  → round: %2d  total: %d",
			r.id, r.names[sc.PlayerID], sc.CardPoints, sc.SpilPoints, sc.TablaPoints, sc.Total,
			r.state.Players[playerIndex(r.state, sc.PlayerID)].TotalScore+sc.Total)
	}
	r.broadcast(protocol.MustMarshal(protocol.RoundEndMsg{
		Type:   "ROUND_END",
		Scores: scores,
	}))
	if r.storage != nil {
		r.storage.RecordRound(r.id, r.state.RoundNumber, scores)
	}

	if winnerIdx := r.state.CheckWinCondition(); winnerIdx >= 0 {
		r.state.Phase = game.PhaseGameOver
		// Build finalPlayers with this round's scores applied (ApplyRoundScores is not called for the final round)
		finalPlayers := make([]game.PublicPlayer, len(r.state.Players))
		for i, p := range r.state.Players {
			pp := p.ToPublic()
			pp.TotalScore = p.TotalScore + scores[i].Total
			finalPlayers[i] = pp
		}
		winner := finalPlayers[winnerIdx]
		log.Printf("[room %s] ── GAME OVER — winner: %s with %d points ──",
			r.id, winner.Name, winner.TotalScore)
		if r.storage != nil {
			r.storage.RecordGameEnd(r.id, winner.Name, finalPlayers)
		}
		r.broadcast(protocol.MustMarshal(protocol.GameOverMsg{
			Type:    "GAME_OVER",
			Winner:  winner,
			Players: finalPlayers,
		}))
		return
	}

	// Start next round
	r.state.ApplyRoundScores(scores)
	if err := r.state.DealNextHand(); err != nil {
		log.Printf("[room %s] next-round deal error: %v", r.id, err)
		return
	}
	log.Printf("[room %s] NEXT ROUND (round #%d)", r.id, r.state.RoundNumber)
	r.logState("new round deal")
	r.broadcast(protocol.MustMarshal(protocol.GameStartedMsg{
		Type:  "GAME_STARTED",
		State: r.state.ToClientState(),
	}))
	for _, p := range r.state.Players {
		r.sendTo(p.ID, protocol.MustMarshal(protocol.HandDealtMsg{
			Type:  "HAND_DEALT",
			Cards: p.Hand,
		}))
	}
	r.broadcast(protocol.MustMarshal(protocol.TurnStartMsg{
		Type:        "TURN_START",
		PlayerIndex: r.state.CurrentPlayerIndex,
	}))
	r.maybeStartTurnTimer()
}

// --- messaging helpers (all called with mu held) ---

func (r *Room) sendTo(playerID string, data []byte) {
	if pc, ok := r.conns[playerID]; ok {
		select {
		case pc.writeCh <- data:
		default:
			log.Printf("room %s: write channel full for %s, dropping message", r.id, playerID)
		}
	}
}

func (r *Room) broadcast(data []byte) {
	for id := range r.conns {
		r.sendTo(id, data)
	}
}

func (r *Room) sendError(playerID, code, msg string) {
	r.sendTo(playerID, protocol.MustMarshal(protocol.ErrorMsg{
		Type:    "ERROR",
		Code:    code,
		Message: msg,
	}))
}

func (r *Room) writeLoop(pc *playerConn) {
	defer pc.conn.Close()
	for {
		select {
		case msg := <-pc.writeCh:
			if err := pc.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-pc.done:
			return
		}
	}
}

// BroadcastGameStart sends GAME_STARTED, private HAND_DEALT, and TURN_START after StartGame().
// All player connections must be attached before calling this.
func (r *Room) BroadcastGameStart() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		return
	}
	log.Printf("[room %s] ── GAME START (round %d, deal %d) ───────────────────", r.id, r.state.RoundNumber, r.state.DealNumber)
	r.logState("game start")
	if r.storage != nil {
		r.storage.RecordGameStart(r.id, r.id, r.state.Players)
	}
	r.broadcast(protocol.MustMarshal(protocol.GameStartedMsg{
		Type:  "GAME_STARTED",
		State: r.state.ToClientState(),
	}))
	for _, p := range r.state.Players {
		r.sendTo(p.ID, protocol.MustMarshal(protocol.HandDealtMsg{
			Type:  "HAND_DEALT",
			Cards: p.Hand,
		}))
	}
	r.broadcast(protocol.MustMarshal(protocol.TurnStartMsg{
		Type:        "TURN_START",
		PlayerIndex: r.state.CurrentPlayerIndex,
	}))
}

// SendErrorTo sends an ERROR message to a specific player (for external callers).
func (r *Room) SendErrorTo(playerID, code, msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sendError(playerID, code, msg)
}

// isConnected reports whether playerID has an open WebSocket. Must be called with mu held.
func (r *Room) isConnected(playerID string) bool {
	pc, ok := r.conns[playerID]
	if !ok {
		return false
	}
	select {
	case <-pc.done:
		return false
	default:
		return pc.conn != nil
	}
}

func (r *Room) cancelTurnTimer() {
	if r.turnTimer != nil {
		r.turnTimer.Stop()
		r.turnTimer = nil
	}
}

// maybeStartTurnTimer starts a 30s auto-skip if the current player is offline.
// Must be called with mu held (releases and re-acquires mu via AfterFunc goroutine).
func (r *Room) maybeStartTurnTimer() {
	r.cancelTurnTimer()
	if r.state == nil {
		return
	}
	cur := r.state.CurrentPlayer()
	if r.isConnected(cur.ID) {
		return
	}
	playerID := cur.ID
	log.Printf("[room %s] turn timer started: %s is offline", r.id, r.names[playerID])
	r.turnTimer = time.AfterFunc(30*time.Second, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.autoSkipTurn(playerID)
	})
}

// autoSkipTurn discards the first card from playerID's hand. Must be called with mu held.
func (r *Room) autoSkipTurn(playerID string) {
	if r.state == nil {
		return
	}
	cur := r.state.CurrentPlayer()
	if cur.ID != playerID || len(cur.Hand) == 0 {
		return
	}
	card := cur.Hand[0] // copy before ApplyDiscard shifts the slice
	log.Printf("[room %s] auto-skip: discarding %s for disconnected player %s", r.id, card.ID, r.names[playerID])
	r.broadcast(protocol.MustMarshal(protocol.CardPlayedMsg{
		Type:     "CARD_PLAYED",
		PlayerID: playerID,
		Card:     card,
	}))
	if err := r.state.ApplyDiscard(playerID, card.ID); err != nil {
		log.Printf("[room %s] auto-skip discard error: %v", r.id, err)
		return
	}
	r.broadcast(protocol.MustMarshal(protocol.CardDiscardedMsg{
		Type: "CARD_DISCARDED",
		Card: card,
	}))
	r.advanceTurnOrDeal()
}

// Disconnect signals a player's write goroutine to stop and notifies others.
func (r *Room) Disconnect(playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	log.Printf("[room %s] player disconnected: %s", r.id, r.names[playerID])
	if pc, ok := r.conns[playerID]; ok && pc.done != nil {
		select {
		case <-pc.done: // already closed
		default:
			close(pc.done)
		}
	}
	r.broadcast(protocol.MustMarshal(protocol.PlayerDisconnectedMsg{
		Type:     "PLAYER_DISCONNECTED",
		PlayerID: playerID,
	}))
}

// --- helpers ---

func flattenGroups(groups [][]game.Card) []game.Card {
	var out []game.Card
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// formatCards returns a short string like "10-diamonds 4-clubs 6-hearts".
func formatCards(cards []game.Card) string {
	ids := make([]string, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	return strings.Join(ids, " ")
}

// formatOptions returns a readable description of capture options.
func formatOptions(options []game.CaptureOption) string {
	parts := make([]string, len(options))
	for i, opt := range options {
		groupStrs := make([]string, len(opt.Groups))
		for j, g := range opt.Groups {
			groupStrs[j] = "[" + shortCards(g) + "]"
		}
		parts[i] = fmt.Sprintf("opt%d:{%s}", i, strings.Join(groupStrs, "+"))
	}
	return strings.Join(parts, " | ")
}

var suitSym = map[game.Suit]string{
	game.Clubs: "♣", game.Diamonds: "♦", game.Hearts: "♥", game.Spades: "♠",
}

// shortCard formats a card as "10♦" or "K♠".
func shortCard(c game.Card) string {
	return string(c.Rank) + suitSym[c.Suit]
}

// shortCards formats a slice of cards as "10♦ K♠ 3♣".
func shortCards(cards []game.Card) string {
	parts := make([]string, len(cards))
	for i, c := range cards {
		parts[i] = shortCard(c)
	}
	return strings.Join(parts, " ")
}

// playerIndex returns the index of a player by ID in the game state (or -1).
func playerIndex(s *game.GameState, playerID string) int {
	for i, p := range s.Players {
		if p.ID == playerID {
			return i
		}
	}
	return -1
}

// logState prints a full snapshot: table, deck count, each player's hand/captured/score.
// Must be called with mu held.
func (r *Room) logState(event string) {
	s := r.state
	if s == nil {
		return
	}
	log.Printf("[room %s] ── after %-20s  Round %d  Deal %d  Deck: %d ──",
		r.id, event, s.RoundNumber, s.DealNumber, len(s.Deck))
	tableStr := shortCards(s.TableCards)
	if tableStr == "" {
		tableStr = "(empty)"
	}
	log.Printf("[room %s]   Table: %s", r.id, tableStr)
	for i, p := range s.Players {
		turn := "  "
		if i == s.CurrentPlayerIndex {
			turn = "▶ "
		}
		handStr := shortCards(p.Hand)
		if handStr == "" {
			handStr = "(empty)"
		}
		log.Printf("[room %s] %s%-12s  hand[%d]: %-36s  cap: %2d  tablas: %d  score: %d",
			r.id, turn, r.names[p.ID], len(p.Hand), handStr,
			len(p.Captured), p.Tablas, p.TotalScore)
	}
}

