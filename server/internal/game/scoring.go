package game

// ComputeTeamRoundScores aggregates scores by team for a 4-player 2v2 game.
// Team assignment: seatIndex % 2 (seats 0,2 = Team 0; seats 1,3 = Team 1).
// Both players on the same team receive identical RoundScore totals.
func ComputeTeamRoundScores(players []Player) []RoundScore {
	type teamData struct {
		captured []Card
		tablas   int
	}
	var teams [2]teamData
	for _, p := range players {
		t := p.SeatIndex % 2
		teams[t].captured = append(teams[t].captured, p.Captured...)
		teams[t].tablas += p.Tablas
	}

	var cardPts [2]int
	for t, td := range teams {
		for _, c := range td.captured {
			cardPts[t] += CardPoint(c)
		}
	}

	var spilPts [2]int
	c0, c1 := len(teams[0].captured), len(teams[1].captured)
	if c0 != c1 {
		if c0 > c1 {
			spilPts[0] = 3
		} else {
			spilPts[1] = 3
		}
	}

	scores := make([]RoundScore, len(players))
	for i, p := range players {
		t := p.SeatIndex % 2
		scores[i] = RoundScore{
			PlayerID:    p.ID,
			CardPoints:  cardPts[t],
			SpilPoints:  spilPts[t],
			TablaPoints: teams[t].tablas,
			Total:       cardPts[t] + spilPts[t] + teams[t].tablas,
		}
	}
	return scores
}

// CardPoint returns the scoring value of a card at round end.
// 10♦ = 2, 2♣ = 1, 10/J/Q/K/A = 1 each, all others = 0.
func CardPoint(c Card) int {
	if c.Rank == Rank10 && c.Suit == Diamonds {
		return 2
	}
	if c.Rank == Rank2 && c.Suit == Clubs {
		return 1
	}
	switch c.Rank {
	case Rank10, RankJ, RankQ, RankK, RankA:
		return 1
	}
	return 0
}

// ComputeRoundScores calculates the RoundScore for each player at round end.
// It does NOT modify any player state.
func ComputeRoundScores(players []Player) []RoundScore {
	scores := make([]RoundScore, len(players))

	// Card points
	for i, p := range players {
		scores[i].PlayerID = p.ID
		for _, c := range p.Captured {
			scores[i].CardPoints += CardPoint(c)
		}
		scores[i].TablaPoints = p.Tablas
	}

	// Špil: 3 pts to the player with the most captured cards (no award on tie)
	maxCards := 0
	for _, p := range players {
		if len(p.Captured) > maxCards {
			maxCards = len(p.Captured)
		}
	}
	winners := 0
	winnerIdx := -1
	for i, p := range players {
		if len(p.Captured) == maxCards {
			winners++
			winnerIdx = i
		}
	}
	if winners == 1 {
		scores[winnerIdx].SpilPoints = 3
	}

	// Total
	for i := range scores {
		scores[i].Total = scores[i].CardPoints + scores[i].SpilPoints + scores[i].TablaPoints
	}

	return scores
}
