package game

import "testing"

// card() helper is defined in capture_test.go (same package).

func twoPlayerGame() *GameState {
	return NewGame([]Player{
		{ID: "p1", Name: "Alice"},
		{ID: "p2", Name: "Bob"},
	}, "test-room")
}

// --- DealNextHand ---

func TestDealNextHand_FirstDeal_PlacesFourTableCards(t *testing.T) {
	gs := twoPlayerGame()
	if err := gs.DealNextHand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gs.TableCards) != 4 {
		t.Errorf("want 4 table cards, got %d", len(gs.TableCards))
	}
	for _, p := range gs.Players {
		if len(p.Hand) != 6 {
			t.Errorf("player %s: want 6 cards, got %d", p.ID, len(p.Hand))
		}
	}
	// 52 - 4 (table) - 12 (2×6) = 36 remaining
	if len(gs.Deck) != 36 {
		t.Errorf("want 36 deck cards remaining, got %d", len(gs.Deck))
	}
	if gs.DealNumber != 1 {
		t.Errorf("want DealNumber=1, got %d", gs.DealNumber)
	}
	if gs.Phase != PhasePlaying {
		t.Errorf("want phase=playing, got %s", gs.Phase)
	}
}

func TestDealNextHand_SubsequentDeal_NoExtraTableCards(t *testing.T) {
	gs := twoPlayerGame()
	_ = gs.DealNextHand() // deal #1 — sets DealNumber=1, places 4 table cards
	// Exhaust hands so a second deal can happen
	for i := range gs.Players {
		gs.Players[i].Hand = []Card{}
	}
	tableLen := len(gs.TableCards)
	_ = gs.DealNextHand() // deal #2
	if len(gs.TableCards) != tableLen {
		t.Errorf("subsequent deal: table grew from %d to %d", tableLen, len(gs.TableCards))
	}
	for _, p := range gs.Players {
		if len(p.Hand) != 6 {
			t.Errorf("player %s: want 6 cards, got %d", p.ID, len(p.Hand))
		}
	}
}

func TestDealNextHand_InsufficientDeck_ReturnsError(t *testing.T) {
	gs := twoPlayerGame()
	gs.DealNumber = 1  // skip table-setup branch
	gs.Deck = gs.Deck[:5] // only 5 cards, need 12 (2×6)
	if err := gs.DealNextHand(); err == nil {
		t.Error("expected error for insufficient deck cards")
	}
}

// --- ApplyDiscard ---

func TestApplyDiscard_MovesCardToTable(t *testing.T) {
	gs := twoPlayerGame()
	c := card(Rank5, Hearts)
	gs.Players[0].Hand = []Card{c}
	gs.TableCards = []Card{}

	if err := gs.ApplyDiscard("p1", c.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gs.Players[0].Hand) != 0 {
		t.Error("card should be removed from hand")
	}
	if len(gs.TableCards) != 1 || gs.TableCards[0].ID != c.ID {
		t.Error("card should be on table")
	}
}

func TestApplyDiscard_UnknownPlayer_ReturnsError(t *testing.T) {
	gs := twoPlayerGame()
	if err := gs.ApplyDiscard("nobody", "5-hearts"); err == nil {
		t.Error("expected error for unknown player")
	}
}

func TestApplyDiscard_CardNotInHand_ReturnsError(t *testing.T) {
	gs := twoPlayerGame()
	gs.Players[0].Hand = []Card{card(Rank5, Hearts)}
	if err := gs.ApplyDiscard("p1", "7-clubs"); err == nil {
		t.Error("expected error for card not in hand")
	}
}

// --- ApplyCapture ---

func TestApplyCapture_FullCapture_IsTabla(t *testing.T) {
	gs := twoPlayerGame()
	played := card(Rank10, Hearts)
	tbl1 := card(Rank4, Clubs)
	tbl2 := card(Rank6, Diamonds)
	gs.Players[0].Hand = []Card{played}
	gs.TableCards = []Card{tbl1, tbl2}

	wasTabla, err := gs.ApplyCapture("p1", played.ID, CaptureOption{Groups: [][]Card{{tbl1, tbl2}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wasTabla {
		t.Error("expected tabla (table fully cleared)")
	}
	if gs.Players[0].Tablas != 1 {
		t.Errorf("want Tablas=1, got %d", gs.Players[0].Tablas)
	}
	if len(gs.TableCards) != 0 {
		t.Error("table should be empty after full capture")
	}
	// played + 2 table cards = 3
	if len(gs.Players[0].Captured) != 3 {
		t.Errorf("want 3 captured, got %d", len(gs.Players[0].Captured))
	}
}

func TestApplyCapture_PartialCapture_IsNotTabla(t *testing.T) {
	gs := twoPlayerGame()
	played := card(Rank5, Spades)
	remaining := card(Rank7, Hearts)
	captured := card(Rank5, Clubs)
	gs.Players[0].Hand = []Card{played}
	gs.TableCards = []Card{remaining, captured}

	wasTabla, err := gs.ApplyCapture("p1", played.ID, CaptureOption{Groups: [][]Card{{captured}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wasTabla {
		t.Error("should not be tabla (card remains on table)")
	}
	if len(gs.TableCards) != 1 || gs.TableCards[0].ID != remaining.ID {
		t.Error("non-captured card should remain on table")
	}
}

func TestApplyCapture_SetsLastCapturerIndex(t *testing.T) {
	gs := twoPlayerGame()
	gs.CurrentPlayerIndex = 1
	played := card(Rank5, Spades)
	tbl := card(Rank5, Clubs)
	gs.Players[1].Hand = []Card{played}
	gs.TableCards = []Card{tbl}

	_, err := gs.ApplyCapture("p2", played.ID, CaptureOption{Groups: [][]Card{{tbl}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gs.LastCapturerIndex == nil || *gs.LastCapturerIndex != 1 {
		t.Errorf("want LastCapturerIndex=1, got %v", gs.LastCapturerIndex)
	}
}

func TestApplyCapture_CardNotInHand_ReturnsError(t *testing.T) {
	gs := twoPlayerGame()
	tbl := card(Rank5, Clubs)
	gs.Players[0].Hand = []Card{card(Rank3, Hearts)}
	gs.TableCards = []Card{tbl}

	_, err := gs.ApplyCapture("p1", "nonexistent", CaptureOption{Groups: [][]Card{{tbl}}})
	if err == nil {
		t.Error("expected error for card not in hand")
	}
}

func TestApplyCapture_CaptureCardNotOnTable_ReturnsError(t *testing.T) {
	gs := twoPlayerGame()
	played := card(Rank5, Spades)
	notOnTable := card(Rank5, Clubs)
	gs.Players[0].Hand = []Card{played}
	gs.TableCards = []Card{} // empty table

	_, err := gs.ApplyCapture("p1", played.ID, CaptureOption{Groups: [][]Card{{notOnTable}}})
	if err == nil {
		t.Error("expected error when capture references card not on table")
	}
}

// --- ApplyLastHandRule ---

func TestApplyLastHandRule_AwardsTableCardsToLastCapturer(t *testing.T) {
	gs := twoPlayerGame()
	tbl := card(Rank7, Hearts)
	gs.TableCards = []Card{tbl}
	idx := 1
	gs.LastCapturerIndex = &idx

	gs.ApplyLastHandRule()

	if len(gs.TableCards) != 0 {
		t.Error("table should be empty after last hand rule")
	}
	if len(gs.Players[1].Captured) != 1 || gs.Players[1].Captured[0].ID != tbl.ID {
		t.Error("last capturer should receive table cards")
	}
}

func TestApplyLastHandRule_NilLastCapturer_DoesNothing(t *testing.T) {
	gs := twoPlayerGame()
	tbl := card(Rank7, Hearts)
	gs.TableCards = []Card{tbl}
	gs.LastCapturerIndex = nil

	gs.ApplyLastHandRule() // must not panic

	if len(gs.TableCards) != 1 {
		t.Error("table cards should remain when no last capturer")
	}
}

func TestApplyLastHandRule_EmptyTable_DoesNothing(t *testing.T) {
	gs := twoPlayerGame()
	gs.TableCards = []Card{}
	idx := 0
	gs.LastCapturerIndex = &idx

	gs.ApplyLastHandRule() // must not panic
}

// --- CheckWinCondition ---

func TestCheckWinCondition_NoWinner(t *testing.T) {
	gs := twoPlayerGame()
	gs.Players[0].TotalScore = 80
	gs.Players[1].TotalScore = 100
	if idx := gs.CheckWinCondition(nil); idx != -1 {
		t.Errorf("want -1 (no winner), got %d", idx)
	}
}

func TestCheckWinCondition_ExactlyAt101_Wins(t *testing.T) {
	gs := twoPlayerGame()
	gs.Players[0].TotalScore = 101
	gs.Players[1].TotalScore = 0
	if idx := gs.CheckWinCondition(nil); idx != 0 {
		t.Errorf("want winner index 0, got %d", idx)
	}
}

func TestCheckWinCondition_BelowThreshold_NoWinner(t *testing.T) {
	gs := twoPlayerGame()
	gs.Players[0].TotalScore = 100
	gs.Players[1].TotalScore = 99
	if idx := gs.CheckWinCondition(nil); idx != -1 {
		t.Errorf("want -1 (100 < 101 threshold), got %d", idx)
	}
}

func TestCheckWinCondition_TieBreak_HighestScoreWins(t *testing.T) {
	gs := twoPlayerGame()
	gs.Players[0].TotalScore = 115
	gs.Players[1].TotalScore = 108
	if idx := gs.CheckWinCondition(nil); idx != 0 {
		t.Errorf("want winner index 0 (115 > 108), got %d", idx)
	}
}

// --- ApplyRoundScores ---

func TestApplyRoundScores_AccumulatesScores(t *testing.T) {
	gs := twoPlayerGame()
	gs.Players[0].TotalScore = 10
	gs.Players[1].TotalScore = 5

	gs.ApplyRoundScores([]RoundScore{
		{PlayerID: "p1", Total: 15},
		{PlayerID: "p2", Total: 7},
	})

	if gs.Players[0].TotalScore != 25 {
		t.Errorf("p1: want 25, got %d", gs.Players[0].TotalScore)
	}
	if gs.Players[1].TotalScore != 12 {
		t.Errorf("p2: want 12, got %d", gs.Players[1].TotalScore)
	}
}

func TestApplyRoundScores_ResetsRoundState(t *testing.T) {
	gs := twoPlayerGame()
	gs.Players[0].Captured = []Card{card(Rank5, Hearts)}
	gs.Players[0].Tablas = 2
	idx := 0
	gs.LastCapturerIndex = &idx
	gs.TableCards = []Card{card(Rank3, Clubs)}
	gs.DealNumber = 5

	gs.ApplyRoundScores([]RoundScore{
		{PlayerID: "p1", Total: 10},
		{PlayerID: "p2", Total: 5},
	})

	if len(gs.Players[0].Captured) != 0 {
		t.Error("captured cards should be reset")
	}
	if gs.Players[0].Tablas != 0 {
		t.Error("tabla count should be reset")
	}
	if gs.LastCapturerIndex != nil {
		t.Error("LastCapturerIndex should be nil after round reset")
	}
	if len(gs.TableCards) != 0 {
		t.Error("table cards should be cleared")
	}
	if gs.DealNumber != 0 {
		t.Errorf("DealNumber should reset to 0, got %d", gs.DealNumber)
	}
	if gs.RoundNumber != 2 {
		t.Errorf("RoundNumber should advance to 2, got %d", gs.RoundNumber)
	}
	if len(gs.Deck) != 52 {
		t.Errorf("deck should be reshuffled to 52 cards, got %d", len(gs.Deck))
	}
}
