/**
 * Tests for configured provider hiding: which collectors run, and which rows
 * survive into the panel.
 */
package limits

import (
	"testing"

	"github.com/senna-lang/herdr-agent-usage/internal/core"
	"github.com/senna-lang/herdr-agent-usage/internal/providers"
)

// countingCollector records whether it ran, so a hidden provider can be shown
// to cost nothing rather than merely being dropped after collection.
func countingCollector(id string, ran *bool) LimitsCollector {
	return func(*string, int64) ProviderLimits {
		*ran = true
		return ProviderLimits{ProviderID: id, Label: id}
	}
}

func collectedIDs(providers []ProviderLimits) []string {
	ids := make([]string, len(providers))
	for i, p := range providers {
		ids[i] = p.ProviderID
	}
	return ids
}

// Hiding a family id hides every account of that family; the other families
// are untouched.
func TestCollectAllProviderLimits_SkipHidesWholeFamily(t *testing.T) {
	var primaryRan, secondaryRan, codexRan bool
	opts := CollectOptions{
		Claude: []ClaudeProfileCollector{
			{ID: "ai_10", Label: "ai_10", Collector: countingCollector("ai_10", &primaryRan)},
			{ID: "personal", Label: "personal", Collector: countingCollector("personal", &secondaryRan)},
		},
		Codex: []CodexProfileCollector{
			{ID: "codex", Label: "Codex", Collector: countingCollector("codex", &codexRan)},
		},
		Skip: HiddenProviderSet([]string{"claude"}),
	}

	got := collectedIDs(CollectAllProviderLimits(nil, 0, opts))
	for _, id := range got {
		if id == "ai_10" || id == "personal" {
			t.Fatalf("hidden family still collected: %v", got)
		}
	}
	if primaryRan || secondaryRan {
		t.Fatal("a hidden provider's collector must not run")
	}
	if !codexRan {
		t.Fatal("hiding one family must not hide another")
	}
}

// Hiding one profile id leaves that family's other accounts visible: the ids
// are how a multi-account user names a single account.
func TestCollectAllProviderLimits_SkipHidesOneProfile(t *testing.T) {
	var primaryRan, secondaryRan bool
	opts := CollectOptions{
		Claude: []ClaudeProfileCollector{
			{ID: "ai_10", Label: "ai_10", Collector: countingCollector("ai_10", &primaryRan)},
			{ID: "personal", Label: "personal", Collector: countingCollector("personal", &secondaryRan)},
		},
		Skip: HiddenProviderSet([]string{"personal"}),
	}

	CollectAllProviderLimits(nil, 0, opts)
	if !primaryRan {
		t.Fatal("unhidden profile must still collect")
	}
	if secondaryRan {
		t.Fatal("hidden profile must not collect")
	}
}

// Skip is applied after Only, so "show every provider" cannot resurrect an id
// the user hid.
func TestCollectAllProviderLimits_SkipSurvivesUnrestrictedCollection(t *testing.T) {
	opts := CollectOptions{
		Grok: []GrokProfileCollector{{ID: "grok", Label: "Grok"}},
		Skip: HiddenProviderSet([]string{"grok"}),
	}

	for _, id := range collectedIDs(CollectAllProviderLimits(nil, 0, opts)) {
		if id == "grok" {
			t.Fatal("hidden provider must stay hidden with Only unset")
		}
	}
}

// Without Skip, nothing changes: every configured family still collects in
// display order.
func TestCollectAllProviderLimits_NoSkipCollectsEveryFamily(t *testing.T) {
	got := collectedIDs(CollectAllProviderLimits(nil, 0, CollectOptions{}))
	want := []string{"claude", "codex", "opencode", "grok", "agy"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("position %d = %q, want %q (display order)", i, got[i], id)
		}
	}
}

func TestFilterHiddenContextPanes(t *testing.T) {
	contextOnly := providers.IDsWithCapability(providers.CapContextOnly)
	if len(contextOnly) < 2 {
		t.Skip("needs two context-only providers to tell hiding apart")
	}
	panes := []ContextPaneUsage{
		{PaneID: "w1:p1", Agent: contextOnly[0], Usage: core.ContextUsage{ContextTokens: 10}},
		{PaneID: "w1:p2", Agent: contextOnly[1], Usage: core.ContextUsage{ContextTokens: 20}},
	}

	got := FilterHiddenContextPanes(panes, HiddenProviderSet([]string{contextOnly[0]}))
	if len(got) != 1 || got[0].Agent != contextOnly[1] {
		t.Fatalf("got %+v, want only %q", got, contextOnly[1])
	}
	if unfiltered := FilterHiddenContextPanes(panes, nil); len(unfiltered) != 2 {
		t.Fatalf("nil hidden set must pass every pane, got %+v", unfiltered)
	}
}
