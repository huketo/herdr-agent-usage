/**
 * Antigravity account allowance as panel blocks.
 *
 * The CLI reports allowance per pool of models that share it: Gemini models
 * have one 5-hour and one weekly window, the third-party models it can also
 * drive (Claude, GPT-OSS) have their own pair. One pool is one panel row, the
 * way one configured account is for the other harnesses, because a pool is
 * exactly the thing that runs out independently.
 *
 * The figures come from the newest fresh statusLine snapshot across every
 * session: quota is billed per Google account, not per conversation, so
 * whichever pane last reported it speaks for the whole account.
 */
package limits

import (
	"github.com/senna-lang/herdr-agent-usage/internal/providers/antigravity"
)

// antigravityWindowMinutes maps the CLI's window names to display lengths.
// The names are the CLI's own vocabulary, so an unknown one is skipped rather
// than guessed into the wrong bar.
var antigravityWindowMinutes = map[string]int{
	"5h":     300,
	"weekly": 10080,
}

// antigravityLabel is the family heading and the single-pool block title.
const antigravityLabel = "Antigravity"

// AntigravityPoolLimits converts one statusLine snapshot into one entry per
// model pool, native pool first. A nil snapshot, or one without quota, yields
// a single entry explaining that no allowance has been observed, which is what
// the panel must say rather than showing a full or empty bar.
func AntigravityPoolLimits(snap *antigravity.Snapshot, nowMs int64) []ProviderLimits {
	var pools []antigravity.QuotaPool
	if snap != nil {
		pools = antigravity.QuotaPools(snap.Quota)
	}
	if len(pools) == 0 {
		note := "no Antigravity statusLine observation yet — see `usagebar setup`"
		return []ProviderLimits{{
			ProviderID:  antigravity.Provider.AgentID(),
			Label:       antigravityLabel,
			Source:      "none",
			FetchedAtMs: nowMs,
			Note:        &note,
		}}
	}

	multiPool := len(pools) > 1
	out := make([]ProviderLimits, 0, len(pools))
	for _, pool := range pools {
		entry := ProviderLimits{
			ProviderID:  antigravityPoolID(pool.ID),
			Label:       antigravityLabel,
			Source:      "antigravity statusLine",
			FetchedAtMs: nowMs,
		}
		if snap.PlanTier != "" {
			plan := snap.PlanTier
			entry.PlanType = &plan
		}
		for _, window := range pool.Windows {
			assignAntigravityWindow(&entry, window)
		}
		if multiPool {
			entry.GroupLabel = antigravityLabel
			entry.AccountLabel = pool.Label
		}
		out = append(out, entry)
	}
	return out
}

// antigravityPoolID derives a stable id from the CLI's pool id ("gemini" ->
// the bare provider id, "3p" -> "agy-3p"). The native pool keeps the bare id so
// a pane's `$limit` and the panel agree on which allowance is "the"
// Antigravity one.
func antigravityPoolID(poolID string) string {
	base := antigravity.Provider.AgentID()
	if poolID == "" || poolID == antigravity.NativePoolID {
		return base
	}
	return base + "-" + poolID
}

// assignAntigravityWindow places one bucket in the slot its window length
// implies: the short window in Primary, the long one in Secondary.
func assignAntigravityWindow(entry *ProviderLimits, window antigravity.PoolWindow) {
	minutes, known := antigravityWindowMinutes[window.Window]
	if !known {
		return
	}

	limit := LimitWindow{
		UsedPercentage: antigravityUsedPercentage(window.RemainingFraction),
		WindowMinutes:  &minutes,
	}
	if !window.ResetsAt.IsZero() {
		resetsAt := window.ResetsAt.Unix()
		limit.ResetsAt = &resetsAt
	}

	switch minutes {
	case antigravityWindowMinutes["5h"]:
		entry.Primary = &limit
	default:
		entry.Secondary = &limit
	}
}

// antigravityUsedPercentage converts a remaining fraction into the used
// percentage the panel renders, clamped: the CLI has been seen to report
// fractions marginally outside 0..1.
func antigravityUsedPercentage(remainingFraction float64) float64 {
	used := (1 - remainingFraction) * 100
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}
