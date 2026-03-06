package game

import (
	"sort"
	"strconv"
	"strings"
)

// CardValues returns the possible numeric values for a card.
// Ace returns [1, 11]; J=12, Q=13, K=14; numeric cards return their face value.
func CardValues(c Card) []int {
	switch c.Rank {
	case RankA:
		return []int{1, 11}
	case RankJ:
		return []int{12}
	case RankQ:
		return []int{13}
	case RankK:
		return []int{14}
	default:
		n, _ := strconv.Atoi(string(c.Rank))
		return []int{n}
	}
}

// FindSubsets returns all subsets of tableCards whose values can sum to target.
// Handles Ace dual-value: an Ace in a subset may contribute 1 or 11.
func FindSubsets(tableCards []Card, target int) [][]Card {
	n := len(tableCards)
	var result [][]Card
	for mask := 1; mask < (1 << n); mask++ {
		var subset []Card
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				subset = append(subset, tableCards[i])
			}
		}
		if subsetCanSum(subset, target) {
			result = append(result, subset)
		}
	}
	return result
}

// subsetCanSum returns true if any combination of card values in the subset sums to target.
func subsetCanSum(subset []Card, target int) bool {
	return subsetSumHelper(subset, 0, 0, target)
}

func subsetSumHelper(cards []Card, idx, currentSum, target int) bool {
	if idx == len(cards) {
		return currentSum == target
	}
	for _, val := range CardValues(cards[idx]) {
		if subsetSumHelper(cards, idx+1, currentSum+val, target) {
			return true
		}
	}
	return false
}

// FindCaptureCombinations returns all valid CaptureOptions from a list of matching subsets.
// A CaptureOption is a set of non-overlapping subsets (no card appears in two groups).
func FindCaptureCombinations(subsets [][]Card) []CaptureOption {
	n := len(subsets)
	var options []CaptureOption
	for mask := 1; mask < (1 << n); mask++ {
		groups, valid := buildNonOverlappingGroups(subsets, n, mask)
		if valid {
			options = append(options, CaptureOption{Groups: groups})
		}
	}
	return options
}

func buildNonOverlappingGroups(subsets [][]Card, n, mask int) ([][]Card, bool) {
	usedCards := make(map[string]bool)
	var groups [][]Card
	for i := 0; i < n; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		for _, c := range subsets[i] {
			if usedCards[c.ID] {
				return nil, false
			}
			usedCards[c.ID] = true
		}
		groups = append(groups, subsets[i])
	}
	return groups, true
}

// ComputeCaptures calculates all valid CaptureOptions when playedCard is played against tableCards.
//
// Rules:
//   - Ace can be used as 1 or 11; all valid captures under either value are offered.
//   - All other cards (2-10, J=12, Q=13, K=14) capture by exact value or sum of table cards.
func ComputeCaptures(playedCard Card, tableCards []Card) CaptureResult {
	if len(tableCards) == 0 {
		return CaptureResult{}
	}

	if playedCard.Rank == RankA {
		// Try Ace as both 1 and 11, union the unique subsets found
		subsets1 := FindSubsets(tableCards, 1)
		subsets11 := FindSubsets(tableCards, 11)
		allSubsets := unionSubsets(subsets1, subsets11)
		options := FindCaptureCombinations(allSubsets)
		return CaptureResult{Options: options}
	}

	// All cards (2–10, J=12, Q=13, K=14): capture by value or sum
	target := CardValues(playedCard)[0]
	subsets := FindSubsets(tableCards, target)
	options := FindCaptureCombinations(subsets)
	return CaptureResult{Options: options}
}

// unionSubsets merges two slice-of-subsets, deduplicating by card ID set.
func unionSubsets(a, b [][]Card) [][]Card {
	seen := make(map[string]bool)
	var result [][]Card
	for _, s := range append(a, b...) {
		key := subsetKey(s)
		if !seen[key] {
			seen[key] = true
			result = append(result, s)
		}
	}
	return result
}

func subsetKey(s []Card) string {
	ids := make([]string, len(s))
	for i, c := range s {
		ids[i] = c.ID
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}
