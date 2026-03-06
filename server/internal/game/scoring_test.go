package game

import "testing"

func TestCardPoint_10ofDiamonds(t *testing.T) {
	c := card(Rank10, Diamonds)
	if CardPoint(c) != 2 {
		t.Errorf("10♦ should be worth 2 points, got %d", CardPoint(c))
	}
}

func TestCardPoint_2ofClubs(t *testing.T) {
	c := card(Rank2, Clubs)
	if CardPoint(c) != 1 {
		t.Errorf("2♣ should be worth 1 point, got %d", CardPoint(c))
	}
}

func TestCardPoint_10nonDiamonds(t *testing.T) {
	for _, suit := range []Suit{Clubs, Hearts, Spades} {
		c := card(Rank10, suit)
		if CardPoint(c) != 1 {
			t.Errorf("10%s should be worth 1 point, got %d", suit, CardPoint(c))
		}
	}
}

func TestCardPoint_FaceCards(t *testing.T) {
	for _, rank := range []Rank{RankJ, RankQ, RankK} {
		c := card(rank, Clubs)
		if CardPoint(c) != 1 {
			t.Errorf("%s should be worth 1 point, got %d", rank, CardPoint(c))
		}
	}
}

func TestCardPoint_Ace(t *testing.T) {
	for _, suit := range allSuits {
		c := card(RankA, suit)
		if CardPoint(c) != 1 {
			t.Errorf("A%s should be worth 1 point, got %d", suit, CardPoint(c))
		}
	}
}

func TestCardPoint_NumericNonScoring(t *testing.T) {
	for _, rank := range []Rank{Rank3, Rank4, Rank5, Rank6, Rank7, Rank8, Rank9} {
		c := card(rank, Hearts)
		if CardPoint(c) != 0 {
			t.Errorf("%s♥ should be worth 0 points, got %d", rank, CardPoint(c))
		}
	}
}

func TestCardPoint_2nonClubs(t *testing.T) {
	for _, suit := range []Suit{Diamonds, Hearts, Spades} {
		c := card(Rank2, suit)
		if CardPoint(c) != 0 {
			t.Errorf("2%s should be 0 points, got %d", suit, CardPoint(c))
		}
	}
}

func TestComputeRoundScores_Basic(t *testing.T) {
	// Player 0: has 10♦, J♥, A♠, 2♣ (2+1+1+1=5 card pts), 30 cards
	// Player 1: has 3♣, 4♥ (0 card pts), 22 cards
	// Player 0 wins špil (30 > 22) → +3
	// No tablas
	p0 := Player{ID: "p0", Captured: []Card{
		card(Rank10, Diamonds), // 2 pts
		card(RankJ, Hearts),    // 1 pt
		card(RankA, Spades),    // 1 pt
		card(Rank2, Clubs),     // 1 pt
	}, Tablas: 0}
	p1 := Player{ID: "p1", Captured: []Card{
		card(Rank3, Clubs),
		card(Rank4, Hearts),
	}, Tablas: 0}

	// Pad player 0 to 30 cards total
	for i := 0; i < 26; i++ {
		p0.Captured = append(p0.Captured, card(Rank5, Hearts))
	}
	// Pad player 1 to 22 cards
	for i := 0; i < 20; i++ {
		p1.Captured = append(p1.Captured, card(Rank6, Clubs))
	}

	scores := ComputeRoundScores([]Player{p0, p1})

	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}

	s0 := scores[0]
	if s0.CardPoints != 5 {
		t.Errorf("p0 card points: expected 5, got %d", s0.CardPoints)
	}
	if s0.SpilPoints != 3 {
		t.Errorf("p0 spil points: expected 3, got %d", s0.SpilPoints)
	}
	if s0.Total != 8 {
		t.Errorf("p0 total: expected 8, got %d", s0.Total)
	}

	s1 := scores[1]
	if s1.CardPoints != 0 {
		t.Errorf("p1 card points: expected 0, got %d", s1.CardPoints)
	}
	if s1.SpilPoints != 0 {
		t.Errorf("p1 spil points: expected 0, got %d", s1.SpilPoints)
	}
}

func TestComputeRoundScores_SpilTie(t *testing.T) {
	// Both players have 26 cards — tie, no špil points awarded
	p0 := Player{ID: "p0", Captured: make([]Card, 26)}
	p1 := Player{ID: "p1", Captured: make([]Card, 26)}

	scores := ComputeRoundScores([]Player{p0, p1})
	for _, s := range scores {
		if s.SpilPoints != 0 {
			t.Errorf("tied špil: expected 0 spil points, got %d for %s", s.SpilPoints, s.PlayerID)
		}
	}
}

func TestComputeRoundScores_TablaPoints(t *testing.T) {
	p0 := Player{ID: "p0", Captured: make([]Card, 30), Tablas: 2}
	p1 := Player{ID: "p1", Captured: make([]Card, 22), Tablas: 1}

	scores := ComputeRoundScores([]Player{p0, p1})
	if scores[0].TablaPoints != 2 {
		t.Errorf("expected 2 tabla pts, got %d", scores[0].TablaPoints)
	}
	if scores[1].TablaPoints != 1 {
		t.Errorf("expected 1 tabla pt, got %d", scores[1].TablaPoints)
	}
}

// ── Team scoring tests ────────────────────────────────────────────────────────

func makePlayer(id string, seatIndex int, captured []Card, tablas int) Player {
	return Player{ID: id, SeatIndex: seatIndex, Captured: captured, Tablas: tablas}
}

func TestComputeTeamRoundScores_BasicCapture(t *testing.T) {
	// Team A (seats 0,2): combined 10♦ + 2♣ = 3 card pts, 30 cards
	// Team B (seats 1,3): 0 card pts, 22 cards
	// Team A gets špil (30 > 22)
	p0 := makePlayer("p0", 0, []Card{card(Rank10, Diamonds), card(Rank2, Clubs)}, 0)
	p2 := makePlayer("p2", 2, make([]Card, 28), 0)
	p1 := makePlayer("p1", 1, make([]Card, 11), 0)
	p3 := makePlayer("p3", 3, make([]Card, 11), 0)

	scores := ComputeTeamRoundScores([]Player{p0, p1, p2, p3})

	// Both Team A players should have card pts = 3 (10♦=2, 2♣=1)
	if scores[0].CardPoints != 3 {
		t.Errorf("p0 (team A) card pts: expected 3, got %d", scores[0].CardPoints)
	}
	if scores[2].CardPoints != 3 {
		t.Errorf("p2 (team A) card pts: expected 3, got %d", scores[2].CardPoints)
	}
	// Team A wins špil (30 cards vs 22)
	if scores[0].SpilPoints != 3 {
		t.Errorf("p0 (team A) spil: expected 3, got %d", scores[0].SpilPoints)
	}
	if scores[2].SpilPoints != 3 {
		t.Errorf("p2 (team A) spil: expected 3, got %d", scores[2].SpilPoints)
	}
	// Team B gets nothing
	if scores[1].CardPoints != 0 || scores[1].SpilPoints != 0 {
		t.Errorf("p1 (team B) should score 0, got card=%d spil=%d", scores[1].CardPoints, scores[1].SpilPoints)
	}
}

func TestComputeTeamRoundScores_SpilTiedNoAward(t *testing.T) {
	// Both teams have 26 cards — tie, no špil
	p0 := makePlayer("p0", 0, make([]Card, 13), 0)
	p2 := makePlayer("p2", 2, make([]Card, 13), 0)
	p1 := makePlayer("p1", 1, make([]Card, 13), 0)
	p3 := makePlayer("p3", 3, make([]Card, 13), 0)

	scores := ComputeTeamRoundScores([]Player{p0, p1, p2, p3})
	for _, s := range scores {
		if s.SpilPoints != 0 {
			t.Errorf("tied špil: expected 0, got %d for %s", s.SpilPoints, s.PlayerID)
		}
	}
}

func TestComputeTeamRoundScores_TablaAggregated(t *testing.T) {
	// p0 has 1 tabla, p2 has 2 tablas → Team A total = 3
	// p1 has 1 tabla, p3 has 0 tablas → Team B total = 1
	p0 := makePlayer("p0", 0, make([]Card, 14), 1)
	p2 := makePlayer("p2", 2, make([]Card, 14), 2)
	p1 := makePlayer("p1", 1, make([]Card, 12), 1)
	p3 := makePlayer("p3", 3, make([]Card, 12), 0)

	scores := ComputeTeamRoundScores([]Player{p0, p1, p2, p3})

	if scores[0].TablaPoints != 3 {
		t.Errorf("p0 team A tabla pts: expected 3, got %d", scores[0].TablaPoints)
	}
	if scores[2].TablaPoints != 3 {
		t.Errorf("p2 team A tabla pts: expected 3, got %d", scores[2].TablaPoints)
	}
	if scores[1].TablaPoints != 1 {
		t.Errorf("p1 team B tabla pts: expected 1, got %d", scores[1].TablaPoints)
	}
	if scores[3].TablaPoints != 1 {
		t.Errorf("p3 team B tabla pts: expected 1, got %d", scores[3].TablaPoints)
	}
}

func TestComputeTeamRoundScores_TeammatesEqualTotal(t *testing.T) {
	// Verify both players on each team receive identical Total
	p0 := makePlayer("p0", 0, []Card{card(Rank10, Diamonds)}, 1) // 2 card pts + 1 tabla
	p2 := makePlayer("p2", 2, []Card{card(RankA, Hearts)}, 0)    // 1 card pt (team A combined: 3)
	p1 := makePlayer("p1", 1, []Card{card(Rank5, Clubs)}, 0)
	p3 := makePlayer("p3", 3, make([]Card, 30), 0) // team B wins špil

	scores := ComputeTeamRoundScores([]Player{p0, p1, p2, p3})

	if scores[0].Total != scores[2].Total {
		t.Errorf("teammates must have equal total: p0=%d p2=%d", scores[0].Total, scores[2].Total)
	}
	if scores[1].Total != scores[3].Total {
		t.Errorf("teammates must have equal total: p1=%d p3=%d", scores[1].Total, scores[3].Total)
	}
}

func TestComputeRoundScores_TotalDeckPoints(t *testing.T) {
	// Full deck distributed: total card points should be 25
	// (10♦=2, 2♣=1, 10s×3=3, Jacks×4=4, Queens×4=4, Kings×4=4, Aces×4=4, 10♦ already counted)
	// Actually: 4 Aces(4) + 4 Jacks(4) + 4 Queens(4) + 4 Kings(4) + 4 tens(4) + 10♦ extra(+1) + 2♣(+1) = 22+3 = 25? Let me count:
	// scoring cards: 10,J,Q,K,A = 5 ranks × 4 suits = 20 cards = 20 pts base
	// 10♦ gets +1 extra = 21
	// 2♣ = +1 = 22  ... hmm that's only 22 not 25
	// Wait: 10s: 4×1 = 4. Plus 10♦ bonus: total for 10s = 4 + 1 = 5.
	// J: 4×1=4, Q: 4×1=4, K: 4×1=4, A: 4×1=4
	// 2♣: 1
	// Total = 5+4+4+4+4+1 = 22? But rules say 25 including špil(3). 22 card pts + 3 špil = 25. ✓
	deck := NewDeck()
	p0 := Player{ID: "p0", Captured: deck[:26]}
	p1 := Player{ID: "p1", Captured: deck[26:]}

	scores := ComputeRoundScores([]Player{p0, p1})

	totalCardPts := scores[0].CardPoints + scores[1].CardPoints
	if totalCardPts != 22 {
		t.Errorf("expected 22 total card points in full deck, got %d", totalCardPts)
	}
	// špil goes to one player
	totalSpil := scores[0].SpilPoints + scores[1].SpilPoints
	if totalSpil != 3 && totalSpil != 0 {
		t.Errorf("špil should be 3 (one winner) or 0 (tie), got %d", totalSpil)
	}
}
