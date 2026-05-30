// Package scheduler provides channel selection via priority+weight+health scoring.
package scheduler

import (
	"math/rand/v2"
	"sort"

	"ai-platform/internal/relay/common"
)

// Select picks the best channel candidate.
// Algorithm:
// 1. Exclude channels with health < 20
// 2. Group by priority, pick highest priority group
// 3. Within group, weighted random selection with health degradation
// 4. If all unhealthy, fall back to all candidates
func Select(candidates []common.ChannelCandidate) *common.ChannelCandidate {
	if len(candidates) == 0 {
		return nil
	}

	healthy := make([]common.ChannelCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.HealthScore >= 20 {
			healthy = append(healthy, c)
		}
	}

	if len(healthy) == 0 {
		healthy = candidates
	}

	groups := groupByPriority(healthy)
	highest := groups[len(groups)-1]
	if len(highest) == 1 {
		return &highest[0]
	}

	return weightedRandomSelect(highest)
}

func groupByPriority(candidates []common.ChannelCandidate) [][]common.ChannelCandidate {
	if len(candidates) == 0 {
		return nil
	}

	sorted := make([]common.ChannelCandidate, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	var groups [][]common.ChannelCandidate
	curPrio := sorted[0].Priority
	curGroup := []common.ChannelCandidate{sorted[0]}

	for i := 1; i < len(sorted); i++ {
		if sorted[i].Priority == curPrio {
			curGroup = append(curGroup, sorted[i])
		} else {
			groups = append(groups, curGroup)
			curPrio = sorted[i].Priority
			curGroup = []common.ChannelCandidate{sorted[i]}
		}
	}
	groups = append(groups, curGroup)
	return groups
}

func weightedRandomSelect(candidates []common.ChannelCandidate) *common.ChannelCandidate {
	total := 0
	weights := make([]int, len(candidates))
	for i, c := range candidates {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		if c.HealthScore < 80 {
			if c.HealthScore >= 50 {
				w /= 2
			} else {
				w /= 4
			}
		}
		weights[i] = w
		total += w
	}

	if total <= 0 {
		return &candidates[0]
	}

	r := rand.IntN(total)
	cum := 0
	for i, w := range weights {
		cum += w
		if r < cum {
			return &candidates[i]
		}
	}
	return &candidates[len(candidates)-1]
}
