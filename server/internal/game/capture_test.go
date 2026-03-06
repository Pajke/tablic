package game

import (
	"sort"
	"testing"
)

// --- helpers ---

func card(rank Rank, suit Suit) Card {
	return Card{ID: string(rank) + "-" + string(suit), Rank: rank, Suit: suit}
}

func cardIDs(cards []Card) []string {
	ids := make([]string, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	sort.Strings(ids)
	return ids
}

func optionCount(result CaptureResult) int {
	return len(result.Options)
}

// --- CardValues ---

func TestCardValues_Numeric(t *testing.T) {
	c := card(Rank7, Hearts)
	vals := CardValues(c)
	if len(vals) != 1 || vals[0] != 7 {
		t.Errorf("expected [7], got %v", vals)
	}
}

func TestCardValues_Jack(t *testing.T) {
	vals := CardValues(card(RankJ, Spades))
	if len(vals) != 1 || vals[0] != 12 {
		t.Errorf("expected [12], got %v", vals)
	}
}

func TestCardValues_Queen(t *testing.T) {
	vals := CardValues(card(RankQ, Spades))
	if len(vals) != 1 || vals[0] != 13 {
		t.Errorf("expected [13], got %v", vals)
	}
}

func TestCardValues_King(t *testing.T) {
	vals := CardValues(card(RankK, Spades))
	if len(vals) != 1 || vals[0] != 14 {
		t.Errorf("expected [14], got %v", vals)
	}
}

func TestCardValues_Ace(t *testing.T) {
	vals := CardValues(card(RankA, Hearts))
	if len(vals) != 2 {
		t.Fatalf("expected 2 values for Ace, got %v", vals)
	}
	sort.Ints(vals)
	if vals[0] != 1 || vals[1] != 11 {
		t.Errorf("expected [1, 11], got %v", vals)
	}
}

// --- FindSubsets ---

func TestFindSubsets_EmptyTable(t *testing.T) {
	result := FindSubsets([]Card{}, 5)
	if len(result) != 0 {
		t.Errorf("expected no subsets on empty table, got %d", len(result))
	}
}

func TestFindSubsets_SingleCardMatch(t *testing.T) {
	table := []Card{card(Rank7, Clubs), card(Rank3, Hearts)}
	subsets := FindSubsets(table, 7)
	if len(subsets) != 1 {
		t.Fatalf("expected 1 subset, got %d", len(subsets))
	}
	if subsets[0][0].Rank != Rank7 {
		t.Errorf("expected 7-clubs subset, got %v", subsets[0])
	}
}

func TestFindSubsets_SumMatch(t *testing.T) {
	// 4 + 6 = 10
	table := []Card{card(Rank4, Clubs), card(Rank6, Diamonds), card(Rank3, Hearts)}
	subsets := FindSubsets(table, 10)
	if len(subsets) != 1 {
		t.Fatalf("expected 1 subset (4+6), got %d: %v", len(subsets), subsets)
	}
}

func TestFindSubsets_MultipleMatches(t *testing.T) {
	// Table: [10, 4, 6] — playing 10 can match [10♠] or [4+6]
	table := []Card{
		card(Rank10, Spades),
		card(Rank4, Clubs),
		card(Rank6, Diamonds),
	}
	subsets := FindSubsets(table, 10)
	if len(subsets) != 2 {
		t.Errorf("expected 2 subsets ([10] and [4,6]), got %d: %v", len(subsets), subsets)
	}
}

func TestFindSubsets_NoMatch(t *testing.T) {
	table := []Card{card(Rank3, Clubs), card(Rank5, Hearts)}
	subsets := FindSubsets(table, 10)
	if len(subsets) != 0 {
		t.Errorf("expected 0 subsets, got %d", len(subsets))
	}
}

func TestFindSubsets_AceOnTable_As1(t *testing.T) {
	// Ace + 9 = 10 (Ace as 1)
	table := []Card{card(RankA, Spades), card(Rank9, Hearts)}
	subsets := FindSubsets(table, 10)
	// Should find [A♠, 9♥] with Ace=1
	if len(subsets) == 0 {
		t.Error("expected A+9=10 subset (Ace as 1), got none")
	}
}

func TestFindSubsets_AceOnTable_As11(t *testing.T) {
	// Ace alone = 11
	table := []Card{card(RankA, Clubs), card(Rank5, Diamonds)}
	subsets := FindSubsets(table, 11)
	found := false
	for _, s := range subsets {
		if len(s) == 1 && s[0].Rank == RankA {
			found = true
		}
	}
	if !found {
		t.Error("expected Ace alone as 11 to be a valid subset")
	}
}

func TestFindSubsets_ThreeCardSum(t *testing.T) {
	// 5 + 2 + 3 = 10
	table := []Card{card(Rank5, Hearts), card(Rank2, Clubs), card(Rank3, Diamonds)}
	subsets := FindSubsets(table, 10)
	if len(subsets) != 1 {
		t.Fatalf("expected 1 subset (5+2+3), got %d", len(subsets))
	}
	if len(subsets[0]) != 3 {
		t.Errorf("expected 3-card subset, got %d cards", len(subsets[0]))
	}
}

// --- FindCaptureCombinations ---

func TestFindCaptureCombinations_SingleSubset(t *testing.T) {
	subsets := [][]Card{{card(Rank10, Hearts)}}
	options := FindCaptureCombinations(subsets)
	if len(options) != 1 {
		t.Errorf("expected 1 option, got %d", len(options))
	}
	if len(options[0].Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(options[0].Groups))
	}
}

func TestFindCaptureCombinations_TwoNonOverlapping(t *testing.T) {
	// [10♠] and [4♣, 6♦] are non-overlapping → can capture both simultaneously
	subsets := [][]Card{
		{card(Rank10, Spades)},
		{card(Rank4, Clubs), card(Rank6, Diamonds)},
	}
	options := FindCaptureCombinations(subsets)
	// Expect: [[10♠]], [[4♣,6♦]], [[10♠],[4♣,6♦]]
	if len(options) != 3 {
		t.Errorf("expected 3 options (each alone + both together), got %d", len(options))
	}
	// Find the multi-capture option
	var multiCapture *CaptureOption
	for i := range options {
		if len(options[i].Groups) == 2 {
			multiCapture = &options[i]
		}
	}
	if multiCapture == nil {
		t.Error("expected one option with 2 groups (multi-capture), found none")
	}
}

func TestFindCaptureCombinations_OverlappingSubsets(t *testing.T) {
	// [3♣, 3♥] and [3♣, 6♦] share 3♣ — cannot be combined
	subsets := [][]Card{
		{card(Rank3, Clubs), card(Rank3, Hearts)},
		{card(Rank3, Clubs), card(Rank6, Diamonds)},
	}
	options := FindCaptureCombinations(subsets)
	// Only individual options — no multi-capture because they overlap on 3♣
	for _, o := range options {
		if len(o.Groups) > 1 {
			t.Errorf("overlapping subsets should not produce multi-capture options, got %v", o)
		}
	}
}

// --- ComputeCaptures ---

func TestComputeCaptures_EmptyTable(t *testing.T) {
	result := ComputeCaptures(card(Rank7, Hearts), []Card{})
	if len(result.Options) != 0 {
		t.Errorf("expected 0 options on empty table, got %d", len(result.Options))
	}
}

func TestComputeCaptures_SingleMatch(t *testing.T) {
	table := []Card{card(Rank7, Clubs), card(Rank3, Diamonds)}
	result := ComputeCaptures(card(Rank7, Hearts), table)
	if len(result.Options) != 1 {
		t.Errorf("expected 1 option, got %d", len(result.Options))
	}
}

func TestComputeCaptures_MultiCapture(t *testing.T) {
	// Table: [10♠, 4♣, 6♦], play 10 → can capture [10♠], [4♣+6♦], or both
	table := []Card{
		card(Rank10, Spades),
		card(Rank4, Clubs),
		card(Rank6, Diamonds),
	}
	result := ComputeCaptures(card(Rank10, Hearts), table)
	if len(result.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(result.Options))
	}
}

func TestComputeCaptures_AcePlayed_As1(t *testing.T) {
	// Table has a single Ace → Ace played matches as 1=1
	table := []Card{card(RankA, Clubs)}
	result := ComputeCaptures(card(RankA, Hearts), table)
	if len(result.Options) == 0 {
		t.Error("Ace should capture Ace on table")
	}
}

func TestComputeCaptures_AcePlayed_As11(t *testing.T) {
	// Table has [J♠] (12)? No — 11 ≠ 12.
	// Table has [5♣, 6♦] (5+6=11) → Ace as 11 should capture
	table := []Card{card(Rank5, Clubs), card(Rank6, Diamonds)}
	result := ComputeCaptures(card(RankA, Hearts), table)
	if len(result.Options) == 0 {
		t.Error("Ace (as 11) should capture 5+6=11 on table")
	}
}

func TestComputeCaptures_AcePlayed_NoMatch(t *testing.T) {
	// Table: [3, 4] — neither 1 nor 11 works
	table := []Card{card(Rank3, Clubs), card(Rank4, Diamonds)}
	result := ComputeCaptures(card(RankA, Hearts), table)
	if len(result.Options) != 0 {
		t.Errorf("expected no captures, got %d options", len(result.Options))
	}
}

func TestComputeCaptures_KingCapture(t *testing.T) {
	// King = 14; table has a King
	table := []Card{card(RankK, Clubs), card(Rank5, Hearts)}
	result := ComputeCaptures(card(RankK, Spades), table)
	if len(result.Options) != 1 {
		t.Errorf("expected 1 option (K captures K), got %d", len(result.Options))
	}
}

func TestComputeCaptures_KingCaptureBySumTwoSevens(t *testing.T) {
	// King = 14; table [7♣, 7♦] = 7+7=14 — King should capture by sum
	table := []Card{card(Rank7, Clubs), card(Rank7, Diamonds)}
	result := ComputeCaptures(card(RankK, Spades), table)
	if len(result.Options) == 0 {
		t.Error("King should capture [7+7]=14 by sum")
	}
}

func TestComputeCaptures_KingCaptureJackAndTwo(t *testing.T) {
	// King = 14; table [J♣, 2♦] = 12+2=14 — King captures [J+2]
	table := []Card{card(RankJ, Clubs), card(Rank2, Diamonds)}
	result := ComputeCaptures(card(RankK, Spades), table)
	if len(result.Options) == 0 {
		t.Error("King should capture [J+2]=14 by sum")
	}
}

func TestComputeCaptures_KingCaptureQueenAndAce(t *testing.T) {
	// King = 14; table [Q♣, A♦] = 13+1=14 (Ace as 1) — King captures [Q+A]
	table := []Card{card(RankQ, Clubs), card(RankA, Diamonds)}
	result := ComputeCaptures(card(RankK, Spades), table)
	if len(result.Options) == 0 {
		t.Error("King should capture [Q + A(as 1)]=14 by sum")
	}
}

func TestComputeCaptures_JackCaptureBySum(t *testing.T) {
	// Jack = 12; table [9♣, 3♦] = 9+3=12 — Jack captures by sum
	table := []Card{card(Rank9, Clubs), card(Rank3, Diamonds)}
	result := ComputeCaptures(card(RankJ, Spades), table)
	if len(result.Options) == 0 {
		t.Error("Jack should capture [9+3]=12 by sum")
	}
}
