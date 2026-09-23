/**
 * Antigravity statusLine command behaviour.
 *
 * Antigravity spawns this on every status update, so it must be cheap, must
 * never leave a partial snapshot behind, and must not overwrite a good
 * snapshot with an unusable payload. The rendered line is either stacked below
 * Antigravity's built-in status line (`stack_with_default: true`, the setup
 * this plugin recommends) or replaces it, so it states only what the built-in
 * line does not: the context meter in the herdr $context format.
 */
package antigravity

import (
	"github.com/senna-lang/herdr-agent-usage/internal/core"
)

// RunStatusLineIn records one statusLine payload under stateDir and returns
// the line to render.
//
// Every payload that reports quota refreshes the account snapshot; only one
// that names a conversation is stored per session, since context belongs to a
// conversation and agy reports none before the first turn.
//
// A payload that cannot produce a usable snapshot is reported as an error and
// leaves stored state untouched: an empty, partial, or malformed update must
// never displace the last good observation.
func RunStatusLineIn(stateDir string, payload []byte, paneID string, nowMs int64) (string, error) {
	snap, err := SnapshotFromStatusLine(payload, paneID, nowMs)
	if err != nil {
		return "", err
	}
	if len(snap.Quota) > 0 {
		if err := WriteAccountSnapshot(stateDir, snap); err != nil {
			return "", err
		}
	}
	if snap.SessionID != "" {
		sessionsDir := SessionsDir(stateDir)
		if err := WriteSnapshot(sessionsDir, snap); err != nil {
			return "", err
		}
		PruneStale(sessionsDir, nowMs, SnapshotFreshnessMs)
	}
	return statusLineText(snap), nil
}

// statusLineText renders through the shared context formatter, so
// Antigravity's own status line and the herdr $context row describe usage
// identically. No column budget is applied: this line is not constrained by
// the sidebar's width.
func statusLineText(snap Snapshot) string {
	usage := toContextUsage(snap)
	text := core.FormatUsageStatus(*usage, core.FormatUsageOptions{})
	if snap.Model == "" {
		return text
	}
	return snap.Model + "  " + text
}
