package room

import (
	"encoding/json"
	"fmt"
	"testing"

	"tablic/server/internal/game"
	"tablic/server/internal/protocol"
)

// setupRoom creates a Room with n fake players (no real WebSocket connections).
// The playerConns have open write channels but nil *websocket.Conn.
func setupRoom(n int) (*Room, []string) {
	r := newRoom("test", n, nil)
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("p%d", i+1)
		r.conns[id] = &playerConn{
			playerID: id,
			writeCh:  make(chan []byte, 64),
			done:     make(chan struct{}),
		}
		r.tokens[id] = fmt.Sprintf("tok%d", i+1)
		r.names[id] = fmt.Sprintf("Player%d", i+1)
		r.seatOrder = append(r.seatOrder, id)
		ids[i] = id
	}
	return r, ids
}

// drainMsgs reads all pending messages from a player's write channel.
func drainMsgs(r *Room, playerID string) []map[string]any {
	ch := r.conns[playerID].writeCh
	var msgs []map[string]any
	for {
		select {
		case data := <-ch:
			var m map[string]any
			json.Unmarshal(data, &m) //nolint:errcheck
			msgs = append(msgs, m)
		default:
			return msgs
		}
	}
}

func msgTypes(msgs []map[string]any) []string {
	types := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if t, ok := m["type"].(string); ok {
			types = append(types, t)
		}
	}
	return types
}

func containsType(msgs []map[string]any, typ string) bool {
	for _, t := range msgTypes(msgs) {
		if t == typ {
			return true
		}
	}
	return false
}

// setupPlayingRoom returns a room with a game in progress and both players' hands empty
// (DealNumber=1 to skip initial table-card setup on the next deal).
func setupPlayingRoom() (*Room, []string) {
	r, ids := setupRoom(2)
	r.state = game.NewGame([]game.Player{
		{ID: ids[0], Name: "Player1"},
		{ID: ids[1], Name: "Player2"},
	}, "test")
	r.state.DealNumber = 1 // subsequent deals won't place table cards
	return r, ids
}

// --- Join ---

func TestRoom_Join_AddsPlayerWithCorrectSeat(t *testing.T) {
	r := newRoom("test", 2, nil)
	id, token, seat, err := r.Join("Alice", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" || token == "" {
		t.Error("expected non-empty player id and reconnect token")
	}
	if seat != 0 {
		t.Errorf("first player: want seat 0, got %d", seat)
	}
	_, _, seat2, _ := r.Join("Bob", 1)
	if seat2 != 1 {
		t.Errorf("second player: want seat 1, got %d", seat2)
	}
}

func TestRoom_Join_RoomFull_ReturnsError(t *testing.T) {
	r := newRoom("test", 2, nil)
	r.Join("Alice", 1) //nolint:errcheck
	r.Join("Bob", 1)   //nolint:errcheck
	_, _, _, err := r.Join("Charlie", 1)
	if err == nil {
		t.Error("expected error joining a full room")
	}
}

func TestRoom_Join_AfterGameStarted_ReturnsError(t *testing.T) {
	r, ids := setupRoom(2)
	r.state = game.NewGame([]game.Player{
		{ID: ids[0], Name: "Player1"},
		{ID: ids[1], Name: "Player2"},
	}, "test")
	_, _, _, err := r.Join("LatePlayer", 1)
	if err == nil {
		t.Error("expected error joining after game has started")
	}
}

// --- Reconnect ---

func TestRoom_Reconnect_ValidToken_ReturnsPlayerAndSeat(t *testing.T) {
	r, ids := setupRoom(2)
	r.state = game.NewGame([]game.Player{
		{ID: ids[0], Name: "Player1"},
		{ID: ids[1], Name: "Player2"},
	}, "test")

	id, seat, err := r.Reconnect("tok1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != ids[0] {
		t.Errorf("want player id %s, got %s", ids[0], id)
	}
	if seat != 0 {
		t.Errorf("want seat 0, got %d", seat)
	}
}

func TestRoom_Reconnect_InvalidToken_ReturnsError(t *testing.T) {
	r, ids := setupRoom(2)
	r.state = game.NewGame([]game.Player{
		{ID: ids[0], Name: "Player1"},
		{ID: ids[1], Name: "Player2"},
	}, "test")

	_, _, err := r.Reconnect("bad-token")
	if err == nil {
		t.Error("expected error for invalid reconnect token")
	}
}

func TestRoom_Reconnect_GameNotStarted_ReturnsError(t *testing.T) {
	r, _ := setupRoom(2)
	_, _, err := r.Reconnect("tok1")
	if err == nil {
		t.Error("expected error when game has not started yet")
	}
}

// --- HandleMessage: PLAY_CARD ---

func TestRoom_HandleMessage_NotCurrentPlayer_SendsError(t *testing.T) {
	r, ids := setupPlayingRoom()
	// CurrentPlayerIndex=0, but player 1 (ids[1]) tries to play
	r.HandleMessage(ids[1], "PLAY_CARD", protocol.PlayCardMsg{Type: "PLAY_CARD", CardID: "5-hearts"})

	msgs := drainMsgs(r, ids[1])
	if !containsType(msgs, "ERROR") {
		t.Errorf("expected ERROR message, got types: %v", msgTypes(msgs))
	}
}

func TestRoom_HandleMessage_CardNotInHand_SendsError(t *testing.T) {
	r, ids := setupPlayingRoom()
	r.state.Players[0].Hand = []game.Card{
		{ID: "5-hearts", Rank: "5", Suit: "hearts"},
	}

	r.HandleMessage(ids[0], "PLAY_CARD", protocol.PlayCardMsg{Type: "PLAY_CARD", CardID: "9-clubs"})

	msgs := drainMsgs(r, ids[0])
	if !containsType(msgs, "ERROR") {
		t.Errorf("expected ERROR message, got types: %v", msgTypes(msgs))
	}
}

func TestRoom_HandleMessage_Discard_BroadcastsCardPlayedAndDiscarded(t *testing.T) {
	r, ids := setupPlayingRoom()
	played := game.Card{ID: "5-hearts", Rank: "5", Suit: "hearts"}
	r.state.Players[0].Hand = []game.Card{played}
	// Give player 1 a card so AllHandsEmpty() stays false → simple TURN_START path
	r.state.Players[1].Hand = []game.Card{{ID: "9-clubs", Rank: "9", Suit: "clubs"}}
	r.state.TableCards = []game.Card{} // no captures possible

	r.HandleMessage(ids[0], "PLAY_CARD", protocol.PlayCardMsg{Type: "PLAY_CARD", CardID: played.ID})

	for _, id := range ids {
		msgs := drainMsgs(r, id)
		if !containsType(msgs, "CARD_PLAYED") {
			t.Errorf("player %s: missing CARD_PLAYED; got %v", id, msgTypes(msgs))
		}
		if !containsType(msgs, "CARD_DISCARDED") {
			t.Errorf("player %s: missing CARD_DISCARDED; got %v", id, msgTypes(msgs))
		}
	}
}

func TestRoom_HandleMessage_MultipleCaptures_SendsCaptureOptions(t *testing.T) {
	r, ids := setupPlayingRoom()
	// 10♥ can capture: [10♠] (rank match) or [4♣+6♦] (sum) → 3 options total
	played := game.Card{ID: "10-hearts", Rank: "10", Suit: "hearts"}
	tbl1 := game.Card{ID: "10-spades", Rank: "10", Suit: "spades"}
	tbl2 := game.Card{ID: "4-clubs", Rank: "4", Suit: "clubs"}
	tbl3 := game.Card{ID: "6-diamonds", Rank: "6", Suit: "diamonds"}
	r.state.Players[0].Hand = []game.Card{played}
	r.state.TableCards = []game.Card{tbl1, tbl2, tbl3}

	r.HandleMessage(ids[0], "PLAY_CARD", protocol.PlayCardMsg{Type: "PLAY_CARD", CardID: played.ID})

	msgs := drainMsgs(r, ids[0])
	if !containsType(msgs, "CAPTURE_OPTIONS") {
		t.Errorf("expected CAPTURE_OPTIONS; got %v", msgTypes(msgs))
	}
	if r.pendingCapture == nil {
		t.Error("pendingCapture should be set after CAPTURE_OPTIONS")
	}
}

// --- HandleMessage: CHOOSE_CAPTURE ---

func TestRoom_HandleMessage_ChooseCapture_NoPending_SendsError(t *testing.T) {
	r, ids := setupPlayingRoom()
	r.pendingCapture = nil

	r.HandleMessage(ids[0], "CHOOSE_CAPTURE", protocol.ChooseCaptureMsg{Type: "CHOOSE_CAPTURE", OptionIndex: 0})

	msgs := drainMsgs(r, ids[0])
	if !containsType(msgs, "ERROR") {
		t.Errorf("expected ERROR for no pending capture; got %v", msgTypes(msgs))
	}
}

func TestRoom_HandleMessage_ChooseCapture_InvalidIndex_SendsError(t *testing.T) {
	r, ids := setupPlayingRoom()
	tbl := game.Card{ID: "5-clubs", Rank: "5", Suit: "clubs"}
	played := game.Card{ID: "5-hearts", Rank: "5", Suit: "hearts"}
	r.state.Players[0].Hand = []game.Card{played}
	r.pendingCapture = []game.CaptureOption{
		{Groups: [][]game.Card{{tbl}}},
	}
	r.pendingCardID = played.ID

	r.HandleMessage(ids[0], "CHOOSE_CAPTURE", protocol.ChooseCaptureMsg{Type: "CHOOSE_CAPTURE", OptionIndex: 99})

	msgs := drainMsgs(r, ids[0])
	if !containsType(msgs, "ERROR") {
		t.Errorf("expected ERROR for out-of-range option; got %v", msgTypes(msgs))
	}
}
