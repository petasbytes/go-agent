package windowing

import "github.com/anthropics/anthropic-sdk-go"

// Stats summarizes the result of window preparation.
//
// Fields:
// - Total: estimated tokens for included groups only.
// - Budget: the input token budget used.
// - IncludedGroups: number of groups included.
// - SkippedGroups: total groups minus IncludedGroups.
// - OverBudgetNewest: true when the newest single group alone exceeds Budget.
type Stats struct {
	Total            int
	Budget           int
	IncludedGroups   int
	SkippedGroups    int
	OverBudgetNewest bool
}

// PrepareSendWindow returns a subslice of msgs (oldest→newest) that fits within
// budget using the TokenCounter, without splitting groups.
//
// Rules:
// - Include whole groups scanning newest→oldest while total ≤ budget.
// - If the newest group alone exceeds budget, return an empty window and set OverBudgetNewest.
// - If budget ≤ 0, return an empty window (OverBudgetNewest set when any groups exist).
func PrepareSendWindow(msgs []anthropic.MessageParam, budget int, c TokenCounter) ([]anthropic.MessageParam, Stats) {
	// Base cases
	if len(msgs) == 0 {
		return nil, Stats{Budget: budget}
	}

	groups := GroupBlocks(msgs)

	// Handle no-capacity budget explicitly
	if budget <= 0 {
		stats := Stats{Budget: budget, IncludedGroups: 0, SkippedGroups: len(groups)}
		if len(groups) > 0 {
			stats.OverBudgetNewest = true
		}
		return nil, stats
	}

	// Walk groups newest → oldest and find the earliest included group index.
	// We first compute each group's cost once to avoid re-counting.
	type gCost struct {
		idx  int
		cost int
	}
	costs := make([]gCost, len(groups))
	for i, g := range groups {
		costs[i] = gCost{idx: i, cost: c.CountGroup(g, msgs)}
	}

	total := 0
	included := 0
	startIdx := len(groups) // exclusive sentinel; will be lowered when a group is included

	for gi := len(groups) - 1; gi >= 0; gi-- {
		gc := costs[gi]
		// If no groups have been included yet and the newest group alone exceeds budget,
		// return empty window and mark OverBudgetNewest=true.
		if included == 0 && gc.cost > budget {
			vlogf("reason=over_budget_newest_group budget=%d cost=%d", budget, gc.cost)
			return nil, Stats{
				Total:            0,
				Budget:           budget,
				IncludedGroups:   0,
				SkippedGroups:    len(groups),
				OverBudgetNewest: true,
			}
		}

		if total+gc.cost <= budget {
			total += gc.cost
			included++
			startIdx = gi
			continue
		}

		// If adding this group would exceed budget, stop scanning older groups.
		break
	}

	if included == 0 {
		// There were groups but none could be included (handled above for newest>budget),
		// or there was some other corner case; return empty window within budget.
		return nil, Stats{Total: 0, Budget: budget, IncludedGroups: 0, SkippedGroups: len(groups)}
	}

	// Convert group index to message index start (groups are contiguous and non-overlapping).
	startMsg := groups[startIdx].Start
	window := msgs[startMsg:]

	stats := Stats{
		Total:          total,
		Budget:         budget,
		IncludedGroups: included,
		SkippedGroups:  len(groups) - included,
	}
	return window, stats
}
