/**
 * Tests for Antigravity statusLine setup guidance.
 */
package setup

import (
	"encoding/json"
	"strings"
	"testing"
)

// The snippet is pasted into settings.json, so it must be a valid JSON member
// that agy reads as a stacked command statusLine running the bridge.
func TestAntigravityStatusLineSnippet_IsStackedCommandEntry(t *testing.T) {
	var parsed struct {
		StatusLine struct {
			Type             string `json:"type"`
			Command          string `json:"command"`
			StackWithDefault bool   `json:"stack_with_default"`
		} `json:"statusLine"`
	}
	if err := json.Unmarshal([]byte("{"+AntigravityStatusLineSnippet("/plugin")+"}"), &parsed); err != nil {
		t.Fatalf("snippet is not valid JSON: %v", err)
	}
	got := parsed.StatusLine
	if got.Type != "command" || got.Command != "bash /plugin/bin/run-antigravity-statusline.sh" || !got.StackWithDefault {
		t.Fatalf("statusLine=%+v", got)
	}
}

func TestAntigravitySetupLines_IncludeSnippet(t *testing.T) {
	joined := strings.Join(antigravitySetupLines("/plugin"), "\n")
	if !strings.Contains(joined, AntigravityStatusLineSnippet("/plugin")) {
		t.Fatalf("setup report omits the settings.json snippet:\n%s", joined)
	}
}
