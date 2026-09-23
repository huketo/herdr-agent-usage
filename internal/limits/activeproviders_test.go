/**
 * Tests for ActiveProviderSet: open panes -> set of provider ids to display.
 */
package limits

import "testing"

// isolatePluginConfig points HERDR_PLUGIN_CONFIG_DIR at an empty temp dir so
// ResolvedClaudeProfiles() (invoked whenever a claude pane is present) always
// synthesizes the single default profile here, regardless of the machine
// running the test.
func isolatePluginConfig(t *testing.T) {
	t.Helper()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", t.TempDir())
}

func TestActiveProviderSet_OnlyOpenAgents(t *testing.T) {
	isolatePluginConfig(t)
	panes := []OpenPaneSnapshot{
		{PaneID: "w1:p1", Agent: "claude"},
		{PaneID: "w1:p2", Agent: "claude"},
		{PaneID: "w1:p3", Agent: "grok"},
	}
	got := ActiveProviderSet(panes, CollectOptions{})
	if len(got) != 2 || !got["claude"] || !got["grok"] {
		t.Fatalf("got %v, want {claude, grok}", got)
	}
}

func TestActiveProviderSet_EmptyPanes(t *testing.T) {
	got := ActiveProviderSet(nil, CollectOptions{})
	if got == nil {
		t.Fatal("want non-nil empty set (empty set means: hide all providers)")
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}

func TestActiveProviderSet_IgnoresUnknownAgents(t *testing.T) {
	panes := []OpenPaneSnapshot{
		{PaneID: "w1:p1", Agent: ""},
		{PaneID: "w1:p2", Agent: "shell"},
		{PaneID: "w1:p3", Agent: "codex"},
	}
	got := ActiveProviderSet(panes, CollectOptions{})
	if len(got) != 1 || !got["codex"] {
		t.Fatalf("got %v, want {codex}", got)
	}
}

func TestActiveProviderFilter_FailedPaneQueryFailsOpen(t *testing.T) {
	// When the pane query failed we cannot know what is open: show all
	// providers (nil filter) instead of blanking the panel.
	got := ActiveProviderFilter(nil, false, CollectOptions{})
	if got != nil {
		t.Fatalf("got %v, want nil (= no filtering)", got)
	}
}

func TestActiveProviderFilter_ConfirmedEmptyHidesAll(t *testing.T) {
	got := ActiveProviderFilter(nil, true, CollectOptions{})
	if got == nil || len(got) != 0 {
		t.Fatalf("got %v, want non-nil empty set", got)
	}
}

func TestActiveProviderFilter_OKUsesActiveSet(t *testing.T) {
	isolatePluginConfig(t)
	panes := []OpenPaneSnapshot{{PaneID: "w1:p1", Agent: "claude"}}
	got := ActiveProviderFilter(panes, true, CollectOptions{})
	if len(got) != 1 || !got["claude"] {
		t.Fatalf("got %v, want {claude}", got)
	}
}

func TestActiveProviderSet_CaseInsensitiveAgentIDs(t *testing.T) {
	isolatePluginConfig(t)
	panes := []OpenPaneSnapshot{
		{PaneID: "w1:p1", Agent: "Claude"},
		{PaneID: "w1:p2", Agent: "OPENCODE"},
	}
	got := ActiveProviderSet(panes, CollectOptions{})
	if len(got) != 2 || !got["claude"] || !got["opencode"] {
		t.Fatalf("got %v, want {claude, opencode}", got)
	}
}

// One pane of a family activates every entry that family expands to, so the
// panel can show configured accounts side by side for comparison.
func TestActiveProviderSet_PaneActivatesEveryEntryOfItsFamily(t *testing.T) {
	isolatePluginConfig(t)
	opts := CollectOptions{
		Claude: []ClaudeProfileCollector{{ID: "claude"}, {ID: "claude-secondary"}},
		Codex:  []CodexProfileCollector{{ID: "codex"}, {ID: "dev"}},
	}

	got := ActiveProviderSet([]OpenPaneSnapshot{{PaneID: "w1:p1", Agent: "claude"}}, opts)
	if len(got) != 2 || !got["claude"] || !got["claude-secondary"] {
		t.Fatalf("got %v, want {claude, claude-secondary}", got)
	}

	got = ActiveProviderSet([]OpenPaneSnapshot{{PaneID: "w1:p1", Agent: "codex"}}, opts)
	if len(got) != 2 || !got["codex"] || !got["dev"] {
		t.Fatalf("got %v, want {codex, dev}", got)
	}
}

// An Antigravity pane activates every allowance pool discovered for the
// account, including the pools whose ids the CLI's response decided.
func TestActiveProviderSet_AntigravityPaneActivatesEveryPool(t *testing.T) {
	isolatePluginConfig(t)
	opts := CollectOptions{
		Antigravity: []AntigravityPoolCollector{{ID: "agy"}, {ID: "agy-3p"}},
	}

	got := ActiveProviderSet([]OpenPaneSnapshot{{PaneID: "w1:p1", Agent: "agy"}}, opts)
	if len(got) != 2 || !got["agy"] || !got["agy-3p"] {
		t.Fatalf("got %v, want {agy, agy-3p}", got)
	}
}

func TestActiveAndBillingFilters_RoutedOMPClaudeSurvivesIntersection(t *testing.T) {
	panes := []OpenPaneSnapshot{{PaneID: "omp-claude", Agent: "omp"}}
	active := activeProviderSetWith(panes, func(OpenPaneSnapshot) (string, bool) {
		return "claude", true
	}, map[string][]string{"claude": {"claude"}})
	billing := BillingProviderFilter(panes, true, BillingDeps{
		EntryIDs: []string{"claude"},
		ResolvePane: func(OpenPaneSnapshot) (string, string, bool) {
			return "claude", "omp", true
		},
		PaneMode: func(string, OpenPaneSnapshot) BillingMode { return BillingSubscription },
	})
	got := IntersectFilters(active, billing)
	if !got["claude"] || len(got) != 1 {
		t.Fatalf("active=%#v billing=%#v intersection=%#v", active, billing, got)
	}
}
