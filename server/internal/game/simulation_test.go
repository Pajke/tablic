package game

import (
	"fmt"
	"testing"
)

// greedyPlay picks the first card in the current player's hand.
// If any capture option exists, it chooses the one that captures the most cards.
// Otherwise it discards.
func greedyPlay(gs *GameState) error {
	cur := gs.CurrentPlayer()
	if len(cur.Hand) == 0 {
		return fmt.Errorf("player %s has empty hand on their turn", cur.ID)
	}
	c := cur.Hand[0]
	result := ComputeCaptures(c, gs.TableCards)
	if len(result.Options) == 0 {
		return gs.ApplyDiscard(cur.ID, c.ID)
	}
	best, bestN := 0, 0
	for i, opt := range result.Options {
		n := 0
		for _, g := range opt.Groups {
			n += len(g)
		}
		if n > bestN {
			bestN = n
			best = i
		}
	}
	_, err := gs.ApplyCapture(cur.ID, c.ID, result.Options[best])
	return err
}

// simulateFullGame drives a complete game to completion (a player reaching 101+ points).
// Returns the final GameState (scores NOT yet applied for the last round),
// the winner index, the last round's scores, and how many rounds were played.
func simulateFullGame(players []Player) (gs *GameState, winnerIdx int, lastScores []RoundScore, rounds int, err error) {
	gs = NewGame(players, "sim-room")
	if err = gs.DealNextHand(); err != nil {
		err = fmt.Errorf("initial deal: %w", err)
		return
	}
	for rounds = 1; rounds <= 30; rounds++ {
		// Play out every deal within this round.
		for {
			for !gs.AllHandsEmpty() {
				if err = greedyPlay(gs); err != nil {
					err = fmt.Errorf("round %d play: %w", rounds, err)
					return
				}
				gs.AdvanceTurn()
			}
			if gs.DeckEmpty() {
				break // round is over
			}
			if err = gs.DealNextHand(); err != nil {
				err = fmt.Errorf("round %d redeal: %w", rounds, err)
				return
			}
		}

		gs.ApplyLastHandRule()
		lastScores = gs.ComputeRoundScores()

		if winnerIdx = gs.CheckWinCondition(lastScores); winnerIdx >= 0 {
			return // game over — caller can inspect gs.Players[*].TotalScore + lastScores
		}

		gs.ApplyRoundScores(lastScores)
		if err = gs.DealNextHand(); err != nil {
			err = fmt.Errorf("round %d start-of-new-round deal: %w", rounds, err)
			return
		}
	}
	err = fmt.Errorf("game did not finish within %d rounds", rounds-1)
	return
}

// scoreByID returns a map playerID → round score total from a slice of RoundScores.
func scoreByID(scores []RoundScore) map[string]int {
	m := make(map[string]int, len(scores))
	for _, s := range scores {
		m[s.PlayerID] = s.Total
	}
	return m
}

// --- 2-player game ---

func TestFullGame_2Player_TerminatesWithWinner(t *testing.T) {
	players := []Player{
		{ID: "p1", Name: "Alice"},
		{ID: "p2", Name: "Bob"},
	}
	gs, winnerIdx, lastScores, rounds, err := simulateFullGame(players)
	if err != nil {
		t.Fatalf("simulation error: %v", err)
	}

	// Winner index must be in range.
	if winnerIdx < 0 || winnerIdx >= len(players) {
		t.Fatalf("invalid winner index %d (want 0–%d)", winnerIdx, len(players)-1)
	}

	// Winner's effective score (pre-apply TotalScore + final round) must reach 101+.
	sb := scoreByID(lastScores)
	winner := &gs.Players[winnerIdx]
	effectiveScore := winner.TotalScore + sb[winner.ID]
	if effectiveScore < 101 {
		t.Errorf("winner %s effective score %d < 101", winner.Name, effectiveScore)
	}

	// All scores must be non-negative.
	for _, p := range gs.Players {
		if p.TotalScore < 0 {
			t.Errorf("player %s has negative TotalScore %d", p.Name, p.TotalScore)
		}
	}
	for _, rs := range lastScores {
		if rs.Total < 0 {
			t.Errorf("player %s has negative final round score %d", rs.PlayerID, rs.Total)
		}
	}

	t.Logf("2-player: winner=%s, effective=%d, rounds=%d", winner.Name, effectiveScore, rounds)
}

func TestFullGame_2Player_AllCardsAccountedEachRound(t *testing.T) {
	// Run a full game and check card totals at end-of-round (before ApplyRoundScores).
	players := []Player{
		{ID: "p1", Name: "Alice"},
		{ID: "p2", Name: "Bob"},
	}
	gs := NewGame(players, "sim-room")
	if err := gs.DealNextHand(); err != nil {
		t.Fatal(err)
	}

	for round := 1; round <= 30; round++ {
		for {
			for !gs.AllHandsEmpty() {
				if err := greedyPlay(gs); err != nil {
					t.Fatalf("round %d: %v", round, err)
				}
				gs.AdvanceTurn()
			}
			if gs.DeckEmpty() {
				break
			}
			if err := gs.DealNextHand(); err != nil {
				t.Fatalf("round %d redeal: %v", round, err)
			}
		}

		gs.ApplyLastHandRule()

		// After last-hand rule: deck=0, all hands=0.
		// captured across all players + table cards should equal 52.
		total := len(gs.TableCards)
		for _, p := range gs.Players {
			if len(p.Hand) != 0 {
				t.Errorf("round %d: player %s still has cards in hand after last-hand rule", round, p.ID)
			}
			total += len(p.Captured)
		}
		if total != 52 {
			t.Errorf("round %d: card total = %d (want 52)", round, total)
		}

		scores := gs.ComputeRoundScores()
		if winnerIdx := gs.CheckWinCondition(scores); winnerIdx >= 0 {
			return // done
		}
		gs.ApplyRoundScores(scores)

		if err := gs.DealNextHand(); err != nil {
			t.Fatalf("round %d new-round deal: %v", round, err)
		}
	}
	t.Fatal("game did not finish within 30 rounds")
}

func TestFullGame_2Player_ScoresNeverDecrease(t *testing.T) {
	players := []Player{
		{ID: "p1", Name: "Alice"},
		{ID: "p2", Name: "Bob"},
	}
	gs := NewGame(players, "sim-room")
	if err := gs.DealNextHand(); err != nil {
		t.Fatal(err)
	}

	prevTotal := make(map[string]int)

	for round := 1; round <= 30; round++ {
		for {
			for !gs.AllHandsEmpty() {
				if err := greedyPlay(gs); err != nil {
					t.Fatalf("round %d: %v", round, err)
				}
				gs.AdvanceTurn()
			}
			if gs.DeckEmpty() {
				break
			}
			if err := gs.DealNextHand(); err != nil {
				t.Fatal(err)
			}
		}

		gs.ApplyLastHandRule()
		scores := gs.ComputeRoundScores()

		if winnerIdx := gs.CheckWinCondition(scores); winnerIdx >= 0 {
			// Verify effective final scores are non-decreasing.
			sb := scoreByID(scores)
			for _, p := range gs.Players {
				effective := p.TotalScore + sb[p.ID]
				if effective < prevTotal[p.ID] {
					t.Errorf("player %s score decreased: %d → %d", p.ID, prevTotal[p.ID], effective)
				}
			}
			return
		}

		gs.ApplyRoundScores(scores)

		for _, p := range gs.Players {
			if p.TotalScore < prevTotal[p.ID] {
				t.Errorf("round %d: player %s TotalScore decreased from %d to %d",
					round, p.ID, prevTotal[p.ID], p.TotalScore)
			}
			prevTotal[p.ID] = p.TotalScore
		}

		if err := gs.DealNextHand(); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("game did not finish within 30 rounds")
}

// --- 4-player game ---

func TestFullGame_4Player_TerminatesWithWinner(t *testing.T) {
	players := []Player{
		{ID: "p1", Name: "Alice"},
		{ID: "p2", Name: "Bob"},
		{ID: "p3", Name: "Carol"},
		{ID: "p4", Name: "Dave"},
	}
	gs, winnerIdx, lastScores, rounds, err := simulateFullGame(players)
	if err != nil {
		t.Fatalf("simulation error: %v", err)
	}

	if winnerIdx < 0 || winnerIdx >= len(players) {
		t.Fatalf("invalid winner index %d (want 0–%d)", winnerIdx, len(players)-1)
	}

	sb := scoreByID(lastScores)
	winner := &gs.Players[winnerIdx]
	effectiveScore := winner.TotalScore + sb[winner.ID]
	if effectiveScore < 101 {
		t.Errorf("winner %s effective score %d < 101", winner.Name, effectiveScore)
	}

	t.Logf("4-player: winner=%s (seat %d), effective=%d, rounds=%d",
		winner.Name, winner.SeatIndex, effectiveScore, rounds)
}

func TestFullGame_4Player_TeamScoresMatch(t *testing.T) {
	// In 4-player mode, team mates (seats 0+2, 1+3) must always receive equal round scores.
	players := []Player{
		{ID: "p1", Name: "Alice"},
		{ID: "p2", Name: "Bob"},
		{ID: "p3", Name: "Carol"},
		{ID: "p4", Name: "Dave"},
	}
	gs := NewGame(players, "sim-room")
	if err := gs.DealNextHand(); err != nil {
		t.Fatal(err)
	}

	for round := 1; round <= 30; round++ {
		for {
			for !gs.AllHandsEmpty() {
				if err := greedyPlay(gs); err != nil {
					t.Fatalf("round %d: %v", round, err)
				}
				gs.AdvanceTurn()
			}
			if gs.DeckEmpty() {
				break
			}
			if err := gs.DealNextHand(); err != nil {
				t.Fatal(err)
			}
		}

		gs.ApplyLastHandRule()
		scores := gs.ComputeRoundScores()
		sb := scoreByID(scores)

		// Team A: p1 (seat 0) and p3 (seat 2) must have equal scores.
		if sb["p1"] != sb["p3"] {
			t.Errorf("round %d: team A scores differ: p1=%d p3=%d", round, sb["p1"], sb["p3"])
		}
		// Team B: p2 (seat 1) and p4 (seat 3) must have equal scores.
		if sb["p2"] != sb["p4"] {
			t.Errorf("round %d: team B scores differ: p2=%d p4=%d", round, sb["p2"], sb["p4"])
		}
		// Round scores must be non-negative.
		for _, rs := range scores {
			if rs.Total < 0 {
				t.Errorf("round %d: player %s negative score %d", round, rs.PlayerID, rs.Total)
			}
		}

		if gs.CheckWinCondition(scores) >= 0 {
			return
		}
		gs.ApplyRoundScores(scores)

		if err := gs.DealNextHand(); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("game did not finish within 30 rounds")
}

func TestFullGame_4Player_AllCardsAccountedEachRound(t *testing.T) {
	players := []Player{
		{ID: "p1", Name: "Alice"},
		{ID: "p2", Name: "Bob"},
		{ID: "p3", Name: "Carol"},
		{ID: "p4", Name: "Dave"},
	}
	gs := NewGame(players, "sim-room")
	if err := gs.DealNextHand(); err != nil {
		t.Fatal(err)
	}

	for round := 1; round <= 30; round++ {
		for {
			for !gs.AllHandsEmpty() {
				if err := greedyPlay(gs); err != nil {
					t.Fatalf("round %d: %v", round, err)
				}
				gs.AdvanceTurn()
			}
			if gs.DeckEmpty() {
				break
			}
			if err := gs.DealNextHand(); err != nil {
				t.Fatal(err)
			}
		}

		gs.ApplyLastHandRule()

		total := len(gs.TableCards)
		for _, p := range gs.Players {
			total += len(p.Captured)
		}
		if total != 52 {
			t.Errorf("round %d: card total = %d (want 52)", round, total)
		}

		scores := gs.ComputeRoundScores()
		if gs.CheckWinCondition(scores) >= 0 {
			return
		}
		gs.ApplyRoundScores(scores)

		if err := gs.DealNextHand(); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("game did not finish within 30 rounds")
}
