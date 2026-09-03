/**
 * Tests for the context-only panel section: which panes it covers, and how it
 * renders next to the quota-owning blocks.
 */
package limits

import (
	"strings"
	"testing"

	"github.com/senna-lang/herdr-agent-usage/internal/core"
	"github.com/senna-lang/herdr-agent-usage/internal/providers"
)

func contextUsage(tokens, window int) *core.ContextUsage {
	usage := core.ContextUsage{ContextTokens: tokens}
	if window > 0 {
		usage.WindowTokens = intPtr(window)
	}
	return &usage
}

func fixedContextDeps(usage *core.ContextUsage) ContextPaneDeps {
	return ContextPaneDeps{ResolveUsage: func(OpenPaneSnapshot) *core.ContextUsage { return usage }}
}

// Every provider registered CapContextOnly must reach the section, and no
// quota-owning provider may: its allowance belongs in its own block. The agent
// set is derived from the registry, so a newly registered context-only
// provider is covered here without editing this test.
func TestCollectContextPaneUsage_CoversExactlyTheContextOnlyProviders(t *testing.T) {
	contextOnly := providers.IDsWithCapability(providers.CapContextOnly)
	if len(contextOnly) == 0 {
		t.Fatal("no context-only providers registered; this section would be dead code")
	}

	var panes []OpenPaneSnapshot
	for i, id := range contextOnly {
		panes = append(panes, OpenPaneSnapshot{PaneID: "w1:p" + string(rune('1'+i)), Agent: id, Label: id + "-pane"})
	}
	for _, id := range providers.IDsWithCapability(providers.CapOwnsSubscriptionQuota) {
		panes = append(panes, OpenPaneSnapshot{PaneID: "w1:q-" + id, Agent: id, Label: id + "-pane"})
	}

	got := CollectContextPaneUsage(panes, fixedContextDeps(contextUsage(42352, 256_000)))
	if len(got) != len(contextOnly) {
		t.Fatalf("collected %d rows, want %d: %+v", len(got), len(contextOnly), got)
	}
	for i, id := range contextOnly {
		if got[i].Agent != id {
			t.Fatalf("row %d = %q, want %q (registration order)", i, got[i].Agent, id)
		}
	}
}

// A pane whose occupancy cannot be read is dropped. Showing it as zero would
// claim an empty context for a session that was merely unreadable.
func TestCollectContextPaneUsage_DropsUnresolvedPanes(t *testing.T) {
	contextOnly := providers.IDsWithCapability(providers.CapContextOnly)
	panes := []OpenPaneSnapshot{{PaneID: "w1:p1", Agent: contextOnly[0], Label: "one"}}

	if got := CollectContextPaneUsage(panes, fixedContextDeps(nil)); len(got) != 0 {
		t.Fatalf("got %+v, want no rows", got)
	}
	if got := CollectContextPaneUsage(panes, ContextPaneDeps{}); len(got) != 0 {
		t.Fatalf("got %+v with no resolver, want no rows", got)
	}
}

// Rows must not reshuffle between redraws of the same panes.
func TestCollectContextPaneUsage_StableOrderWithinProvider(t *testing.T) {
	agent := providers.IDsWithCapability(providers.CapContextOnly)[0]
	panes := []OpenPaneSnapshot{
		{PaneID: "w1:p9", Agent: agent, Label: "zeta"},
		{PaneID: "w1:p2", Agent: agent, Label: "alpha"},
		{PaneID: "w1:p1", Agent: agent, Label: "alpha"},
	}

	got := CollectContextPaneUsage(panes, fixedContextDeps(contextUsage(1000, 200_000)))
	want := []string{"w1:p1", "w1:p2", "w1:p9"}
	for i, id := range want {
		if got[i].PaneID != id {
			t.Fatalf("row %d = %q, want %q", i, got[i].PaneID, id)
		}
	}
}

func contextPane(agent, label string, tokens, window int) ContextPaneUsage {
	return ContextPaneUsage{PaneID: "w1:p1", Agent: agent, Label: label, Usage: *contextUsage(tokens, window)}
}

// The section states the pane and the same occupancy string the sidebar's
// $context row shows, so the two surfaces cannot disagree.
func TestFormatUsagePanel_ContextSectionNamesPaneAndOccupancy(t *testing.T) {
	out := FormatUsagePanel(nil, nil, []ContextPaneUsage{contextPane("agy", "herdr-agent-usage", 42352, 256_000)},
		1_800_000_000_000, PanelLayout{Columns: 60, Rows: 40})

	for _, want := range []string{"Context", "agy", "herdr-agent-usage", "17% (42k)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

// A panel holding nothing but context panes is not the empty state.
func TestFormatUsagePanel_ContextOnlyIsNotEmpty(t *testing.T) {
	out := FormatUsagePanel(nil, nil, []ContextPaneUsage{contextPane("cursor", "web", 16000, 200_000)},
		1_800_000_000_000, PanelLayout{Columns: 44, Rows: 40})

	if strings.Contains(out, "no usage data yet") {
		t.Fatalf("context-only panel must not render the empty state:\n%s", out)
	}
}

// Occupancy is not allowance: the section must never precede a provider's
// remaining-window block, which is what the pane exists to show.
func TestFormatUsagePanel_ContextSectionRendersLast(t *testing.T) {
	five := 300
	sub := ProviderLimits{
		ProviderID: "claude", Label: "Claude", PlanType: strPtr("Max"),
		Primary: &LimitWindow{UsedPercentage: 10, WindowMinutes: &five},
	}
	api := APIProviderUsage{BackendID: "deepseek", Label: "DeepSeek",
		Windows: []APIUsageWindow{{WindowMinutes: 1440, Tokens: 600, CostUSD: 0.06}}}

	out := FormatUsagePanel([]ProviderLimits{sub}, []APIProviderUsage{api},
		[]ContextPaneUsage{contextPane("agy", "repo", 42352, 256_000)},
		1_800_000_000_000, PanelLayout{Columns: 60, Rows: 40})

	claudeAt, apiAt, contextAt := strings.Index(out, "Claude"), strings.Index(out, "DeepSeek"), strings.Index(out, "Context")
	if claudeAt < 0 || apiAt < 0 || contextAt < 0 {
		t.Fatalf("a block is missing:\n%s", out)
	}
	if claudeAt >= apiAt || apiAt >= contextAt {
		t.Fatalf("want subscription < api < context, got %d < %d < %d:\n%s", claudeAt, apiAt, contextAt, out)
	}
}

// A short pane collapses the whole section to one line and says how many panes
// it could not name, rather than overflowing the pane.
func TestFormatUsagePanel_ContextSectionCompactsInShortPane(t *testing.T) {
	panes := []ContextPaneUsage{
		contextPane("agy", "alpha", 42352, 256_000),
		contextPane("agy", "beta", 30000, 256_000),
		contextPane("cursor", "gamma", 16000, 200_000),
	}
	five := 300
	sub := ProviderLimits{ProviderID: "claude", Label: "Claude",
		Primary: &LimitWindow{UsedPercentage: 10, WindowMinutes: &five}}

	out := FormatUsagePanel([]ProviderLimits{sub}, nil, panes, 1_800_000_000_000, PanelLayout{Columns: 30, Rows: 6})

	body := strings.Count(out, "Context")
	if body != 1 {
		t.Fatalf("context heading must render once, got %d:\n%s", body, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "Context") {
			continue
		}
		if strings.Contains(line, "gamma") {
			t.Fatalf("compact line must drop panes that do not fit:\n%s", line)
		}
		if !strings.Contains(line, "+") {
			t.Fatalf("compact line must report the dropped panes:\n%s", line)
		}
	}
}

// Rows are width-budgeted: a long pane label is truncated instead of wrapping,
// which would break the panel's alignment.
func TestFormatUsagePanel_ContextRowFitsThePaneWidth(t *testing.T) {
	const columns = 44
	long := contextPane("agy", strings.Repeat("verylongpanelabel", 4), 42352, 256_000)

	out := FormatUsagePanel(nil, nil, []ContextPaneUsage{long}, 1_800_000_000_000, PanelLayout{Columns: columns, Rows: 40})

	for _, line := range strings.Split(out, "\n") {
		if core.DisplayWidth(line) > columns {
			t.Fatalf("line exceeds %d columns (%d): %q", columns, core.DisplayWidth(line), line)
		}
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("truncated label must be marked:\n%s", out)
	}
}
