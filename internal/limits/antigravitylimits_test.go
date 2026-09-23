/**
 * Tests for turning Antigravity's statusLine quota into panel entries: pool
 * ids, window slots, and the "nothing observed yet" case.
 */
package limits

import (
	"testing"
	"time"

	"github.com/senna-lang/herdr-agent-usage/internal/providers/antigravity"
)

// sampleSnapshot mirrors the quota map of a real agy 1.2.8 statusLine payload.
func sampleSnapshot() *antigravity.Snapshot {
	return &antigravity.Snapshot{
		PlanTier: "Google AI Ultra",
		Quota: map[string]antigravity.QuotaWindow{
			"3p-weekly":     {RemainingFraction: 1, ResetTime: "2026-09-30T04:12:17Z"},
			"gemini-weekly": {RemainingFraction: 1, ResetTime: "2026-09-30T03:00:11Z"},
			"3p-5h":         {RemainingFraction: 0.5, ResetTime: "2026-09-23T09:12:17Z"},
			"gemini-5h":     {RemainingFraction: 0.75, ResetTime: "2026-09-23T09:00:38Z"},
		},
	}
}

func TestAntigravityPoolLimits_OnePoolPerEntry(t *testing.T) {
	got := AntigravityPoolLimits(sampleSnapshot(), 1_700_000_000_000)
	if len(got) != 2 {
		t.Fatalf("entries=%d, want one per pool", len(got))
	}

	// The native pool keeps the bare provider id and comes first, so a pane's
	// $limit and the panel agree on which allowance is "the" Antigravity one.
	if got[0].ProviderID != "agy" || got[1].ProviderID != "agy-3p" {
		t.Fatalf("pool ids=%q,%q, want agy,agy-3p", got[0].ProviderID, got[1].ProviderID)
	}
	if got[0].GroupLabel != "Antigravity" || got[0].AccountLabel != "Gemini" {
		t.Fatalf("grouping=%+v", got[0])
	}
	if got[1].AccountLabel != "Claude & GPT" {
		t.Fatalf("third-party label=%q", got[1].AccountLabel)
	}
	if got[0].PlanType == nil || *got[0].PlanType != "Google AI Ultra" {
		t.Fatalf("plan=%v", got[0].PlanType)
	}

	// The 5-hour window is the short one and so the headline.
	five := got[0].Primary
	if five == nil || five.UsedPercentage != 25 || *five.WindowMinutes != 300 {
		t.Fatalf("5h window=%+v, want 25%% used over 300 minutes", five)
	}
	wantReset := time.Date(2026, 9, 23, 9, 0, 38, 0, time.UTC).Unix()
	if five.ResetsAt == nil || *five.ResetsAt != wantReset {
		t.Fatalf("5h resetsAt=%v, want %d", five.ResetsAt, wantReset)
	}
	weekly := got[0].Secondary
	if weekly == nil || weekly.UsedPercentage != 0 || *weekly.WindowMinutes != 10080 {
		t.Fatalf("weekly window=%+v", weekly)
	}
	if got[1].Primary == nil || got[1].Primary.UsedPercentage != 50 {
		t.Fatalf("third-party 5h=%+v, want 50%% used", got[1].Primary)
	}
}

func TestAntigravityPoolLimits_SinglePoolIsNotGrouped(t *testing.T) {
	snap := &antigravity.Snapshot{Quota: map[string]antigravity.QuotaWindow{
		"gemini-weekly": {RemainingFraction: 0.4},
	}}
	got := AntigravityPoolLimits(snap, 0)
	if len(got) != 1 || got[0].GroupLabel != "" || got[0].AccountLabel != "" {
		t.Fatalf("single pool must render as a plain block: %+v", got)
	}
	if got[0].Secondary == nil || got[0].Secondary.ResetsAt != nil {
		t.Fatalf("weekly window without reset_time=%+v", got[0].Secondary)
	}
}

// Nothing observed must say so instead of rendering a full or empty bar.
func TestAntigravityPoolLimits_NoObservation(t *testing.T) {
	for name, snap := range map[string]*antigravity.Snapshot{
		"no snapshot": nil,
		"no quota":    {},
	} {
		got := AntigravityPoolLimits(snap, 5)
		if len(got) != 1 || got[0].Note == nil || got[0].Primary != nil || got[0].Secondary != nil {
			t.Fatalf("%s: got %+v, want one note-only entry", name, got)
		}
		if got[0].ProviderID != "agy" || got[0].Source != "none" {
			t.Fatalf("%s: entry=%+v", name, got[0])
		}
	}
}

// A window name the CLI has not used before is skipped, not guessed into a bar.
func TestAntigravityPoolLimits_UnknownWindowSkipped(t *testing.T) {
	snap := &antigravity.Snapshot{Quota: map[string]antigravity.QuotaWindow{
		"gemini-daily": {RemainingFraction: 0},
	}}
	got := AntigravityPoolLimits(snap, 0)
	if len(got) != 1 || got[0].Primary != nil || got[0].Secondary != nil {
		t.Fatalf("unknown window must not fill a slot: %+v", got)
	}
}
