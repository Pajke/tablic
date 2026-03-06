package game

import "errors"

// NewGame creates a fresh GameState for a new game.
// players must have IDs and Names set; SeatIndex is assigned by position.
func NewGame(players []Player, roomID string) *GameState {
	ps := make([]Player, len(players))
	for i, p := range players {
		ps[i] = p
		ps[i].SeatIndex = i
		ps[i].Hand = []Card{}
		ps[i].Captured = []Card{}
		ps[i].Tablas = 0
	}
	deck := NewDeck()
	Shuffle(deck)
	return &GameState{
		RoomID:             roomID,
		Phase:              PhaseWaiting,
		Players:            ps,
		Deck:               deck,
		TableCards:         []Card{},
		CurrentPlayerIndex: 0,
		LastCapturerIndex:  nil,
		DealNumber:         0,
		RoundNumber:        1,
	}
}

// DealNextHand deals 6 cards to each player from the deck.
// On the first deal (DealNumber == 0), also places 4 cards face-up on the table.
// Returns an error if the deck does not have enough cards.
func (s *GameState) DealNextHand() error {
	perPlayer := 6
	needed := len(s.Players) * perPlayer
	if s.DealNumber == 0 {
		needed += 4
	}
	if len(s.Deck) < needed {
		return errors.New("not enough cards in deck to deal")
	}

	if s.DealNumber == 0 {
		s.TableCards = append(s.TableCards, s.Deck[:4]...)
		s.Deck = s.Deck[4:]
	}

	for i := range s.Players {
		s.Players[i].Hand = append(s.Players[i].Hand, s.Deck[:perPlayer]...)
		s.Deck = s.Deck[perPlayer:]
	}

	s.DealNumber++
	s.Phase = PhasePlaying
	return nil
}

// CurrentPlayer returns a pointer to the player whose turn it is.
func (s *GameState) CurrentPlayer() *Player {
	return &s.Players[s.CurrentPlayerIndex]
}

// ApplyCapture applies a capture to the game state.
// Removes the played card from the player's hand, removes captured cards from the table,
// adds all to the player's captured pile, and checks for tabla.
// Returns true if the capture resulted in a tabla (table cleared).
// Returns an error if the player doesn't have the played card or the capture is invalid.
func (s *GameState) ApplyCapture(playerID string, playedCardID string, opt CaptureOption) (bool, error) {
	pi, err := s.findPlayer(playerID)
	if err != nil {
		return false, err
	}
	p := &s.Players[pi]

	// Find and remove the played card from hand
	playedCard, err := removeCardFromHand(p, playedCardID)
	if err != nil {
		return false, err
	}

	// Collect all cards being captured
	capturedIDs := make(map[string]bool)
	for _, group := range opt.Groups {
		for _, c := range group {
			capturedIDs[c.ID] = true
		}
	}

	// Remove captured cards from table
	var remaining []Card
	var captured []Card
	for _, tc := range s.TableCards {
		if capturedIDs[tc.ID] {
			captured = append(captured, tc)
		} else {
			remaining = append(remaining, tc)
		}
	}
	if len(captured) != len(capturedIDs) {
		return false, errors.New("capture references cards not on table")
	}

	s.TableCards = remaining
	p.Captured = append(p.Captured, playedCard)
	p.Captured = append(p.Captured, captured...)

	idx := pi
	s.LastCapturerIndex = &idx

	// Tabla: table is now empty
	wasTabla := len(s.TableCards) == 0
	if wasTabla {
		p.Tablas++
	}

	return wasTabla, nil
}

// ApplyDiscard moves a card from the player's hand to the table (no capture).
func (s *GameState) ApplyDiscard(playerID string, cardID string) error {
	pi, err := s.findPlayer(playerID)
	if err != nil {
		return err
	}
	p := &s.Players[pi]

	discarded, err := removeCardFromHand(p, cardID)
	if err != nil {
		return err
	}
	s.TableCards = append(s.TableCards, discarded)
	return nil
}

// AdvanceTurn moves to the next player in seat order.
func (s *GameState) AdvanceTurn() {
	s.CurrentPlayerIndex = (s.CurrentPlayerIndex + 1) % len(s.Players)
}

// AllHandsEmpty returns true if every player's hand is empty.
func (s *GameState) AllHandsEmpty() bool {
	for _, p := range s.Players {
		if len(p.Hand) > 0 {
			return false
		}
	}
	return true
}

// DeckEmpty returns true if no cards remain in the deck.
func (s *GameState) DeckEmpty() bool {
	return len(s.Deck) == 0
}

// ApplyLastHandRule gives all remaining table cards to the last player who made a capture.
// Must be called when the final hand is exhausted and the deck is empty.
func (s *GameState) ApplyLastHandRule() {
	if len(s.TableCards) == 0 || s.LastCapturerIndex == nil {
		return
	}
	p := &s.Players[*s.LastCapturerIndex]
	p.Captured = append(p.Captured, s.TableCards...)
	s.TableCards = []Card{}
}

// ComputeRoundScores calculates scores at the end of a round.
// For 4 players it uses team scoring (2v2); for 2 players it uses individual scoring.
func (s *GameState) ComputeRoundScores() []RoundScore {
	if len(s.Players) == 4 {
		return ComputeTeamRoundScores(s.Players)
	}
	return ComputeRoundScores(s.Players)
}

// ApplyRoundScores adds round scores to each player's total and resets round state.
// Shuffles a new deck and resets deal counter for the next round.
func (s *GameState) ApplyRoundScores(scores []RoundScore) {
	scoreByID := make(map[string]int, len(scores))
	for _, rs := range scores {
		scoreByID[rs.PlayerID] = rs.Total
	}
	for i := range s.Players {
		s.Players[i].TotalScore += scoreByID[s.Players[i].ID]
		s.Players[i].Captured = []Card{}
		s.Players[i].Tablas = 0
		s.Players[i].Hand = []Card{}
	}
	s.Deck = NewDeck()
	Shuffle(s.Deck)
	s.TableCards = []Card{}
	s.LastCapturerIndex = nil
	s.DealNumber = 0
	s.RoundNumber++
}

// CheckWinCondition returns the winning player's index if any player has reached 101+ points,
// or -1 if the game should continue. If multiple players hit 101+ in the same round,
// the highest score wins (returns that player's index).
func (s *GameState) CheckWinCondition() int {
	maxScore := 0
	winner := -1
	anyOver101 := false
	for i, p := range s.Players {
		if p.TotalScore >= 101 {
			anyOver101 = true
		}
		if p.TotalScore > maxScore {
			maxScore = p.TotalScore
			winner = i
		}
	}
	if anyOver101 {
		return winner
	}
	return -1
}

// ToClientState returns the public game state (no deck, no opponent hands).
func (s *GameState) ToClientState() ClientGameState {
	players := make([]PublicPlayer, len(s.Players))
	for i, p := range s.Players {
		players[i] = p.ToPublic()
	}
	return ClientGameState{
		RoomID:             s.RoomID,
		Phase:              s.Phase,
		Players:            players,
		TableCards:         s.TableCards,
		CurrentPlayerIndex: s.CurrentPlayerIndex,
		LastCapturerIndex:  s.LastCapturerIndex,
		DealNumber:         s.DealNumber,
		RoundNumber:        s.RoundNumber,
		TeamMode:           len(s.Players) == 4,
	}
}

// --- helpers ---

func (s *GameState) findPlayer(playerID string) (int, error) {
	for i, p := range s.Players {
		if p.ID == playerID {
			return i, nil
		}
	}
	return -1, errors.New("player not found: " + playerID)
}

func removeCardFromHand(p *Player, cardID string) (Card, error) {
	for i, c := range p.Hand {
		if c.ID == cardID {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			return c, nil
		}
	}
	return Card{}, errors.New("card not in hand: " + cardID)
}
