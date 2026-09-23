/**
 * Tests for the Antigravity statusLine command behaviour.
 */
package antigravity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const capturedSessionID = "11111111-2222-3333-4444-555555555555"

// preTurnPayload is the shape agy 1.2.8 sends before the first turn: no
// conversation yet, but the account's quota is already known.
const preTurnPayload = `{
  "conversation_id": "",
  "model": { "id": "Gemini 3.8 Flash (Low)", "display_name": "Gemini 3.8 Flash (Low)" },
  "context_window": { "total_input_tokens": 0, "total_output_tokens": 0, "context_window_size": 1048576, "current_usage": null },
  "quota": {
    "gemini-5h": { "remaining_fraction": 0.9920175, "reset_time": "2026-09-23T09:00:38Z" },
    "gemini-weekly": { "remaining_fraction": 0.9995503, "reset_time": "2026-09-30T03:00:11Z" }
  },
  "plan_tier": "Google AI Ultra"
}`

func TestRunStatusLineIn_RecordsAndRenders(t *testing.T) {
	stateDir := t.TempDir()
	text, err := RunStatusLineIn(stateDir, []byte(capturedPayload), "w1:p1", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text == "" {
		t.Fatal("rendered no status line text")
	}
	snap, err := ReadSnapshot(SessionsDir(stateDir), capturedSessionID)
	if err != nil {
		t.Fatalf("snapshot not recorded: %v", err)
	}
	if snap.ContextTokens != 18558 || snap.PaneID != "w1:p1" {
		t.Fatalf("recorded %+v", snap)
	}
	account, ok := LatestAccountSnapshot(stateDir, now)
	if !ok || account.PlanTier != "Antigravity Starter Quota" || len(account.Quota) != 2 {
		t.Fatalf("account quota not recorded: ok=%v %+v", ok, account)
	}
}

// Before the first turn there is no conversation to store context under, but
// the account's quota is valid and must reach the panel, and agy still needs a
// line to render.
func TestRunStatusLineIn_PreTurnRecordsAccountQuotaOnly(t *testing.T) {
	stateDir := t.TempDir()
	text, err := RunStatusLineIn(stateDir, []byte(preTurnPayload), "w1:p1", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(text, "Gemini 3.8 Flash (Low)") {
		t.Fatalf("rendered %q", text)
	}
	if got := ListSnapshots(SessionsDir(stateDir)); len(got) != 0 {
		t.Fatalf("pre-turn payload wrote session snapshots: %+v", got)
	}
	account, ok := LatestAccountSnapshot(stateDir, now)
	if !ok || account.Quota["gemini-5h"].RemainingFraction != 0.9920175 {
		t.Fatalf("account quota=%+v ok=%v", account, ok)
	}
}

// The central guarantee: an unusable update must leave the last good snapshots
// exactly as they were, rather than blanking or truncating them.
func TestRunStatusLineIn_BadPayloadPreservesLastGoodSnapshot(t *testing.T) {
	stateDir := t.TempDir()
	if _, err := RunStatusLineIn(stateDir, []byte(capturedPayload), "w1:p1", now); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Compare encoded bytes: Snapshot holds pointer/map fields, so struct
	// equality would compare identity rather than the recorded values.
	files := []string{
		filepath.Join(SessionsDir(stateDir), capturedSessionID+".json"),
		filepath.Join(stateDir, accountFileName),
	}
	before := make([]string, len(files))
	for i, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("seed read: %v", err)
		}
		before[i] = string(raw)
	}

	for _, payload := range []string{
		"",
		"   ",
		"garbage",
		`{"conversation_id":"` + capturedSessionID + `","context_window":{`,
		`{"conversation_id":"` + capturedSessionID + `"}`,
	} {
		if _, err := RunStatusLineIn(stateDir, []byte(payload), "w1:p1", now+1); err == nil {
			t.Fatalf("payload %q: expected an error", payload)
		}
		for i, path := range files {
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("payload %q: %s became unreadable: %v", payload, path, err)
			}
			if string(after) != before[i] {
				t.Fatalf("payload %q: %s changed from %s to %s", payload, path, before[i], after)
			}
		}
	}
}

// A payload without quota (an older agy, or a signed-out one) must not erase
// the account quota another pane last reported.
func TestRunStatusLineIn_NoQuotaKeepsAccountSnapshot(t *testing.T) {
	stateDir := t.TempDir()
	if _, err := RunStatusLineIn(stateDir, []byte(preTurnPayload), "w1:p1", now); err != nil {
		t.Fatalf("seed: %v", err)
	}
	noQuota := `{"conversation_id":"c2","context_window":{"total_input_tokens":5}}`
	if _, err := RunStatusLineIn(stateDir, []byte(noQuota), "w1:p2", now+1); err != nil {
		t.Fatalf("run: %v", err)
	}
	account, ok := LatestAccountSnapshot(stateDir, now+1)
	if !ok || len(account.Quota) != 2 {
		t.Fatalf("account quota was replaced: %+v", account)
	}
}

func TestRunStatusLineIn_PrunesStaleSnapshots(t *testing.T) {
	stateDir := t.TempDir()
	dir := SessionsDir(stateDir)
	stale := Snapshot{SessionID: "old", PaneID: "w1:p1", ContextTokens: 1, UpdatedAtMs: now - SnapshotFreshnessMs - 1}
	recent := Snapshot{SessionID: "recent", PaneID: "w1:p1", ContextTokens: 2, UpdatedAtMs: now - 1000}
	for _, snap := range []Snapshot{stale, recent} {
		if err := WriteSnapshot(dir, snap); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if _, err := RunStatusLineIn(stateDir, []byte(capturedPayload), "w1:p1", now); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := ReadSnapshot(dir, "old"); err == nil {
		t.Error("stale snapshot was not pruned")
	}
	if _, err := ReadSnapshot(dir, "recent"); err != nil {
		t.Errorf("still-resolvable snapshot was pruned: %v", err)
	}
}

// Antigravity renders stdout as the status line, so the text must describe
// the same usage the herdr $context row does.
func TestStatusLineText_UsesSharedContextFormatting(t *testing.T) {
	text := statusLineText(Snapshot{Model: "Gemini 3.8 Flash (High)", ContextTokens: 18558, WindowTokens: windowOf(1048576)})
	if text != "Gemini 3.8 Flash (High)  ⛁ 2% (19k)" {
		t.Fatalf("got %q", text)
	}
	// Without a model label the line is the context text alone.
	if got := statusLineText(Snapshot{ContextTokens: 900}); got != "⛁ 900" {
		t.Fatalf("got %q", got)
	}
}
