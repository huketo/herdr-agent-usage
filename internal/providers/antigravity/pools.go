/**
 * Antigravity allowance pools.
 *
 * The statusLine quota map is flat: one entry per bucket, keyed
 * "<pool>-<window>" ("gemini-5h", "3p-weekly"). The CLI's own `/quota` command
 * groups the same buckets into pools of models that share an allowance, and a
 * pool is what runs out independently, so this regroups the flat map the same
 * way. Pool and window names are the CLI's vocabulary: a pool Google adds
 * appears on its own under its raw id, and one it removes disappears instead
 * of rendering an empty bar.
 */
package antigravity

import (
	"sort"
	"strings"
	"time"
)

// NativePoolID is the pool of Antigravity's own models. It is listed first,
// since it is the allowance the CLI's default model draws on.
const NativePoolID = "gemini"

// poolLabels names the pools observed so far: short forms of the group names
// `agy -p /quota` prints ("Gemini Models", "Claude and GPT models"), since a
// pool row shares the narrow panel line with its bars. The statusLine payload
// carries bucket ids only.
var poolLabels = map[string]string{
	NativePoolID: "Gemini",
	"3p":         "Claude & GPT",
}

// QuotaPool is one pool of models that share an allowance.
type QuotaPool struct {
	// ID is the CLI's pool prefix ("gemini", "3p").
	ID string
	// Label is the pool's display name; the raw ID when the pool is unknown.
	Label   string
	Windows []PoolWindow
}

// PoolWindow is one allowance window of one pool.
type PoolWindow struct {
	// BucketID is the CLI's own bucket id ("gemini-5h").
	BucketID string
	// Window is the CLI's window name ("5h", "weekly").
	Window string
	// RemainingFraction is 0..1 of the allowance still available.
	RemainingFraction float64
	// ResetsAt is when the window fully refreshes. Zero when not reported or
	// unparseable.
	ResetsAt time.Time
}

// QuotaPools groups a snapshot's quota buckets into pools: the native pool
// first, then the others by id, each pool's windows ordered by bucket id so
// the result is deterministic whatever order the map iterates in.
func QuotaPools(quota map[string]QuotaWindow) []QuotaPool {
	byID := make(map[string]*QuotaPool)
	for bucketID, w := range quota {
		poolID, window := splitBucketID(bucketID)
		pool, ok := byID[poolID]
		if !ok {
			pool = &QuotaPool{ID: poolID, Label: poolLabel(poolID)}
			byID[poolID] = pool
		}
		pool.Windows = append(pool.Windows, PoolWindow{
			BucketID:          bucketID,
			Window:            window,
			RemainingFraction: w.RemainingFraction,
			ResetsAt:          parseResetTime(w.ResetTime),
		})
	}

	pools := make([]QuotaPool, 0, len(byID))
	for _, pool := range byID {
		sort.Slice(pool.Windows, func(i, j int) bool { return pool.Windows[i].BucketID < pool.Windows[j].BucketID })
		pools = append(pools, *pool)
	}
	sort.Slice(pools, func(i, j int) bool {
		if (pools[i].ID == NativePoolID) != (pools[j].ID == NativePoolID) {
			return pools[i].ID == NativePoolID
		}
		return pools[i].ID < pools[j].ID
	})
	return pools
}

// splitBucketID splits "<pool>-<window>" at the last dash, since a window name
// never contains one while a future pool id might. An id without a dash is a
// pool with an unknown window, which callers skip rather than guess.
func splitBucketID(bucketID string) (pool, window string) {
	idx := strings.LastIndex(bucketID, "-")
	if idx <= 0 {
		return bucketID, ""
	}
	return bucketID[:idx], bucketID[idx+1:]
}

func poolLabel(poolID string) string {
	if label, ok := poolLabels[poolID]; ok {
		return label
	}
	return poolID
}

// parseResetTime reads the bucket's absolute reset instant. reset_in_seconds
// is ignored on purpose: it is only accurate at capture time.
func parseResetTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
