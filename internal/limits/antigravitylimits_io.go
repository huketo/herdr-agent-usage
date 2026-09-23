/**
 * Production wiring for the Antigravity allowance collector.
 *
 * The snapshot is read once per collection pass and shared by every pool's
 * collector: one observation carries the whole account, so reading again per
 * pool would only repeat the same directory scan.
 */
package limits

import (
	"time"

	"github.com/senna-lang/herdr-agent-usage/internal/providers/antigravity"
)

// AntigravityPoolCollectors returns one collector per model pool in the newest
// fresh statusLine snapshot, shaped like the other families' per-account
// collectors so the panel treats them the same way.
func AntigravityPoolCollectors() []AntigravityPoolCollector {
	pools := AntigravityPoolLimits(latestAntigravitySnapshot(time.Now().UnixMilli()), 0)
	collectors := make([]AntigravityPoolCollector, len(pools))
	for i, pool := range pools {
		limits := pool
		collectors[i] = AntigravityPoolCollector{
			ID:    limits.ProviderID,
			Label: limits.Label,
			Collector: func(_ *string, nowMs int64) ProviderLimits {
				limits.FetchedAtMs = nowMs
				return limits
			},
		}
	}
	return collectors
}

// latestAntigravitySnapshot is nil when the state directory cannot be located
// or no fresh snapshot exists.
func latestAntigravitySnapshot(nowMs int64) *antigravity.Snapshot {
	stateDir := antigravity.StateDir()
	if stateDir == "" {
		return nil
	}
	snap, ok := antigravity.LatestFreshSnapshot(antigravity.SessionsDir(stateDir), nowMs)
	if !ok {
		return nil
	}
	return &snap
}
