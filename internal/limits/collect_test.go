/**
 * Tests for CollectAllProviderLimits facade.
 */
package limits

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectAllProviderLimits_OrderAndStubs(t *testing.T) {
	got := CollectAllProviderLimits(nil, 100, CollectOptions{})
	if len(got) != 5 {
		t.Fatalf("len=%d", len(got))
	}
	wantIDs := []string{"claude", "codex", "opencode", "grok", "agy"}
	for i, id := range wantIDs {
		if got[i].ProviderID != id {
			t.Fatalf("[%d] id=%q want %q", i, got[i].ProviderID, id)
		}
		if got[i].Note == nil || !containsStr(*got[i].Note, "not configured") {
			t.Fatalf("[%d] expected stub note, got %v", i, got[i].Note)
		}
	}
}

func TestCollectAllProviderLimits_WithCollectorsAndAttach(t *testing.T) {
	cwd := "/tmp"
	got := CollectAllProviderLimits(&cwd, 200, CollectOptions{
		Claude: []ClaudeProfileCollector{{
			ID: "claude", Label: "Claude",
			Collector: func(c *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "claude", Label: "Claude", Source: "test", FetchedAtMs: now}
			},
		}},
		Codex: []CodexProfileCollector{{
			ID: "codex", Label: "Codex",
			Collector: func(c *string, now int64) ProviderLimits {
				if c == nil || *c != "/tmp" {
					t.Fatal("cwd not passed")
				}
				return ProviderLimits{ProviderID: "codex", Label: "Codex", Source: "test", FetchedAtMs: now}
			},
		}},
		Attach: func(providers []ProviderLimits, nowMs int64) []ProviderLimits {
			if nowMs != 200 || len(providers) != 5 {
				t.Fatalf("attach args")
			}
			providers[0].PaneActivity = &ProviderPaneActivity{WindowMinutes: 300, TotalTokens: 1}
			return providers
		},
	})
	if got[0].PaneActivity == nil || got[0].PaneActivity.TotalTokens != 1 {
		t.Fatalf("attach not applied: %+v", got[0])
	}
	if got[1].Source != "test" {
		t.Fatalf("codex=%+v", got[1])
	}
}

func TestCollectAllProviderLimits_OnlyFiltersProviders(t *testing.T) {
	got := CollectAllProviderLimits(nil, 100, CollectOptions{
		Only: map[string]bool{"claude": true, "grok": true},
	})
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].ProviderID != "claude" || got[1].ProviderID != "grok" {
		t.Fatalf("ids=%q,%q want claude,grok (display order kept)", got[0].ProviderID, got[1].ProviderID)
	}
}

func TestCollectAllProviderLimits_OnlyEmptyHidesAll(t *testing.T) {
	got := CollectAllProviderLimits(nil, 100, CollectOptions{Only: map[string]bool{}})
	if len(got) != 0 {
		t.Fatalf("len=%d, want 0", len(got))
	}
}

func TestCollectAllProviderLimits_OnlySkipsFilteredCollectors(t *testing.T) {
	codexCalled := false
	got := CollectAllProviderLimits(nil, 100, CollectOptions{
		Only: map[string]bool{"claude": true},
		Codex: []CodexProfileCollector{{
			ID: "codex", Label: "Codex",
			Collector: func(_ *string, now int64) ProviderLimits {
				codexCalled = true
				return ProviderLimits{ProviderID: "codex", Label: "Codex", Source: "test", FetchedAtMs: now}
			},
		}},
		Attach: func(providers []ProviderLimits, _ int64) []ProviderLimits {
			if len(providers) != 1 {
				t.Fatalf("attach got %d providers, want 1 (filtered)", len(providers))
			}
			return providers
		},
	})
	if codexCalled {
		t.Fatal("codex collector ran despite being filtered out")
	}
	if len(got) != 1 || got[0].ProviderID != "claude" {
		t.Fatalf("got %+v", got)
	}
}

func TestCollectAllProviderLimits_MultipleClaudeProfiles(t *testing.T) {
	got := CollectAllProviderLimits(nil, 100, CollectOptions{
		Claude: []ClaudeProfileCollector{
			{ID: "claude", Label: "Claude", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "claude", Label: "Claude", Source: "test-a", FetchedAtMs: now}
			}},
			{ID: "claude-secondary", Label: "Claude (secondary)", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "claude-secondary", Label: "Claude (secondary)", Source: "test-b", FetchedAtMs: now}
			}},
		},
		Only: map[string]bool{"claude": true, "claude-secondary": true, "codex": true, "opencode": true, "grok": true},
	})
	if len(got) != 5 {
		t.Fatalf("len=%d want 5: %+v", len(got), got)
	}
	if got[0].ProviderID != "claude" || got[0].Source != "test-a" {
		t.Fatalf("profile 1 = %+v", got[0])
	}
	if got[1].ProviderID != "claude-secondary" || got[1].Source != "test-b" {
		t.Fatalf("profile 2 = %+v", got[1])
	}
	if got[2].ProviderID != "codex" {
		t.Fatalf("codex should follow all claude profiles, got %+v", got[2])
	}
}

func TestCollectAllProviderLimits_ClaudeProfileFilteredByOnly(t *testing.T) {
	secondaryCalled := false
	got := CollectAllProviderLimits(nil, 100, CollectOptions{
		Claude: []ClaudeProfileCollector{
			{ID: "claude", Label: "Claude", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "claude", Label: "Claude", Source: "test-a", FetchedAtMs: now}
			}},
			{ID: "claude-secondary", Label: "Claude (secondary)", Collector: func(_ *string, now int64) ProviderLimits {
				secondaryCalled = true
				return ProviderLimits{ProviderID: "claude-secondary", Label: "Claude (secondary)", Source: "test-b", FetchedAtMs: now}
			}},
		},
		Only: map[string]bool{"claude": true},
	})
	if len(got) != 1 || got[0].ProviderID != "claude" {
		t.Fatalf("got %+v", got)
	}
	if secondaryCalled {
		t.Fatal("filtered-out profile's collector must not run")
	}
}

func TestDefaultCollectOptions_MultiProfileGroupsEveryProfileUnderClaude(t *testing.T) {
	pluginConfigDir := t.TempDir()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", pluginConfigDir)
	dirLabeled := t.TempDir()
	dirUnlabeled := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirLabeled, ".claude.json"),
		[]byte(`{"oauthAccount":{"emailAddress":"primary@example.com"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirUnlabeled, ".claude.json"),
		[]byte(`{"oauthAccount":{"emailAddress":"secondary@example.com"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	toml := "[[claude.profiles]]\n" +
		"id = \"claude\"\n" +
		"label = \"My Work Account\"\n" +
		"config_dir = \"" + dirLabeled + "\"\n\n" +
		"[[claude.profiles]]\n" +
		"id = \"claude-secondary\"\n" +
		"config_dir = \"" + dirUnlabeled + "\"\n"
	if err := os.WriteFile(filepath.Join(pluginConfigDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultCollectOptions()
	if len(opts.Claude) != 2 {
		t.Fatalf("want 2 claude collectors, got %d", len(opts.Claude))
	}
	// The explicit label must be preserved as Label; AccountLabel carries the
	// real email so the panel can show it instead once grouped.
	if opts.Claude[0].Label != "My Work Account" {
		t.Fatalf("explicit label must be preserved, got %q", opts.Claude[0].Label)
	}
	got0 := opts.Claude[0].Collector(nil, 0)
	if got0.GroupLabel != "Claude" {
		t.Fatalf("labeled profile should be grouped under Claude, got %q", got0.GroupLabel)
	}
	if got0.AccountLabel != "primary@example.com" {
		t.Fatalf("labeled profile should still resolve AccountLabel from its email, got %q", got0.AccountLabel)
	}
	// The unlabeled profile keeps its id as Label, plus the same grouping.
	if opts.Claude[1].Label != "claude-secondary" {
		t.Fatalf("unlabeled profile label should default to id, got %q", opts.Claude[1].Label)
	}
	got1 := opts.Claude[1].Collector(nil, 0)
	if got1.GroupLabel != "Claude" {
		t.Fatalf("unlabeled profile should be grouped under Claude, got %q", got1.GroupLabel)
	}
	if got1.AccountLabel != "secondary@example.com" {
		t.Fatalf("unlabeled profile should resolve AccountLabel from its email, got %q", got1.AccountLabel)
	}
}

func TestDefaultCollectOptions_SingleProfileNotGrouped(t *testing.T) {
	pluginConfigDir := t.TempDir()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", pluginConfigDir)
	// Isolate from the real machine's ~/.claude.json (age/content varies by
	// machine and would otherwise make this test's Note assertion flaky).
	t.Setenv("HOME", t.TempDir())
	// No [[claude.profiles]] configured -> single synthesized default; grouping
	// only kicks in once there are 2+ profiles to disambiguate.
	opts := DefaultCollectOptions()
	if len(opts.Claude) != 1 {
		t.Fatalf("want 1 (default) claude collector, got %d", len(opts.Claude))
	}
	got := opts.Claude[0].Collector(nil, 0)
	if got.GroupLabel != "" {
		t.Fatalf("single-profile mode should not set GroupLabel, got %q", got.GroupLabel)
	}
	if got.AccountLabel != "" {
		t.Fatalf("single-profile mode should not set AccountLabel, got %q", got.AccountLabel)
	}
}

func TestCollectAllProviderLimits_MultipleCodexProfiles(t *testing.T) {
	got := CollectAllProviderLimits(nil, 100, CollectOptions{
		Codex: []CodexProfileCollector{
			{ID: "codex", Label: "personal", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "codex", Label: "personal", Source: "test-a", FetchedAtMs: now}
			}},
			{ID: "dev", Label: "product", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "dev", Label: "product", Source: "test-b", FetchedAtMs: now}
			}},
		},
		Only: map[string]bool{"claude": true, "codex": true, "dev": true, "opencode": true, "grok": true},
	})
	if len(got) != 5 {
		t.Fatalf("len=%d want 5: %+v", len(got), got)
	}
	if got[1].ProviderID != "codex" || got[1].Source != "test-a" {
		t.Fatalf("profile 1 = %+v", got[1])
	}
	if got[2].ProviderID != "dev" || got[2].Source != "test-b" {
		t.Fatalf("profile 2 = %+v", got[2])
	}
	if got[3].ProviderID != "opencode" {
		t.Fatalf("opencode should follow all codex profiles, got %+v", got[3])
	}
}

func TestCollectAllProviderLimits_CodexProfileFilteredByOnly(t *testing.T) {
	devCalled := false
	got := CollectAllProviderLimits(nil, 100, CollectOptions{
		Codex: []CodexProfileCollector{
			{ID: "codex", Label: "personal", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "codex", Label: "personal", Source: "test-a", FetchedAtMs: now}
			}},
			{ID: "dev", Label: "product", Collector: func(_ *string, now int64) ProviderLimits {
				devCalled = true
				return ProviderLimits{ProviderID: "dev", Label: "product", Source: "test-b", FetchedAtMs: now}
			}},
		},
		Only: map[string]bool{"codex": true},
	})
	if len(got) != 1 || got[0].ProviderID != "codex" {
		t.Fatalf("got %+v", got)
	}
	if devCalled {
		t.Fatal("filtered-out profile's collector must not run")
	}
}

func TestDefaultCollectOptions_MultiProfileGroupsEveryProfileUnderCodex(t *testing.T) {
	pluginConfigDir := t.TempDir()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", pluginConfigDir)
	t.Setenv("HOME", t.TempDir())
	dirPersonal := t.TempDir()
	dirDev := t.TempDir()
	toml := "[[codex.profiles]]\n" +
		"id = \"codex\"\n" +
		"label = \"personal\"\n" +
		"codex_home = \"" + dirPersonal + "\"\n\n" +
		"[[codex.profiles]]\n" +
		"id = \"dev\"\n" +
		"codex_home = \"" + dirDev + "\"\n"
	if err := os.WriteFile(filepath.Join(pluginConfigDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultCollectOptions()
	if len(opts.Codex) != 2 {
		t.Fatalf("want 2 codex collectors, got %d", len(opts.Codex))
	}
	if opts.Codex[0].Label != "personal" {
		t.Fatalf("explicit label must be preserved, got %q", opts.Codex[0].Label)
	}
	got0 := opts.Codex[0].Collector(nil, 0)
	if got0.GroupLabel != "Codex" {
		t.Fatalf("labeled profile should be grouped under Codex, got %q", got0.GroupLabel)
	}
	if got0.AccountLabel != "personal" {
		t.Fatalf("AccountLabel should be the profile label, got %q", got0.AccountLabel)
	}
	if opts.Codex[1].Label != "dev" {
		t.Fatalf("unlabeled profile label should default to id, got %q", opts.Codex[1].Label)
	}
	got1 := opts.Codex[1].Collector(nil, 0)
	if got1.GroupLabel != "Codex" || got1.AccountLabel != "dev" {
		t.Fatalf("unlabeled profile grouping = %q/%q", got1.GroupLabel, got1.AccountLabel)
	}
}

// OMP rotates among every signed-in Codex account, so its usage_history is the
// only complete local account list. A zero-config panel must show each account,
// rather than refusing the ambiguous borrow and rendering one empty Codex row.
func TestDefaultCollectOptions_DiscoversEveryObservedCodexAccount(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	nowMs := int64(1_788_703_200_000)
	rows := codexObservationRowsForAccount(
		nowMs-60_000,
		"oauth|account:acct-work|email:work@example.com",
		"work@example.com",
		"acct-work",
		0.25,
		0.75,
	)
	rows = append(rows, codexObservationRowsForAccount(
		nowMs-120_000,
		"oauth|account:acct-personal|email:personal@example.com",
		"personal@example.com",
		"acct-personal",
		0.5,
		0.125,
	)...)
	useAgentDB(t, rows...)

	opts := DefaultCollectOptions()
	if len(opts.Codex) != 2 {
		t.Fatalf("Codex collectors = %d, want one per observed account", len(opts.Codex))
	}

	got := map[string]ProviderLimits{}
	ids := map[string]bool{}
	for _, collector := range opts.Codex {
		limits := collector.Collector(nil, nowMs)
		got[limits.AccountLabel] = limits
		if ids[limits.ProviderID] {
			t.Fatalf("duplicate provider id %q", limits.ProviderID)
		}
		ids[limits.ProviderID] = true
	}
	for label, wantPrimary := range map[string]float64{
		"personal@example.com": 50,
		"work@example.com":     25,
	} {
		limits, ok := got[label]
		if !ok {
			t.Fatalf("missing Codex account %q: %+v", label, got)
		}
		if limits.GroupLabel != "Codex" {
			t.Fatalf("%s group = %q, want Codex", label, limits.GroupLabel)
		}
		if limits.Primary == nil || limits.Primary.UsedPercentage != wantPrimary {
			t.Fatalf("%s primary = %+v, want %.0f%%", label, limits.Primary, wantPrimary)
		}
	}
}

// A configured CODEX_HOME and OMP may describe the same account. Its auth
// identity must join those sources into one row while unmatched OMP accounts
// remain visible.
func TestDefaultCollectOptions_DeduplicatesConfiguredAndObservedCodexAccount(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", t.TempDir())
	t.Setenv("HOME", home)
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"),
		[]byte(`{"tokens":{"account_id":"acct-work"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	nowMs := int64(1_788_703_200_000)
	rows := codexObservationRowsForAccount(
		nowMs-60_000,
		"oauth|account:acct-work|email:work@example.com",
		"work@example.com",
		"acct-work",
		0.25,
		0.75,
	)
	rows = append(rows, codexObservationRowsForAccount(
		nowMs-120_000,
		"oauth|account:acct-personal|email:personal@example.com",
		"personal@example.com",
		"acct-personal",
		0.5,
		0.125,
	)...)
	useAgentDB(t, rows...)

	opts := DefaultCollectOptions()
	if len(opts.Codex) != 2 {
		t.Fatalf("Codex collectors = %d, want configured account plus one unmatched account", len(opts.Codex))
	}
	got := make(map[string]ProviderLimits, len(opts.Codex))
	for _, collector := range opts.Codex {
		account := collector.Collector(nil, nowMs)
		got[account.AccountLabel] = account
	}
	if got["work@example.com"].ProviderID != "codex" {
		t.Fatalf("configured account = %+v, want the stable codex provider id", got["work@example.com"])
	}
	if personal := got["personal@example.com"]; personal.ProviderID == "" || personal.ProviderID == "codex" {
		t.Fatalf("observed account = %+v, want its own provider id", personal)
	}
}

func TestCollectAllProviderLimits_MultipleGrokAndOpenCodeProfiles(t *testing.T) {
	got := CollectAllProviderLimits(nil, 100, CollectOptions{
		Grok: []GrokProfileCollector{
			{ID: "grok-personal", Label: "Personal", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "grok-personal", Label: "Personal", Source: "grok-personal", FetchedAtMs: now}
			}},
			{ID: "grok-work", Label: "Work", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "grok-work", Label: "Work", Source: "grok-work", FetchedAtMs: now}
			}},
		},
		OpenCode: []OpenCodeProfileCollector{
			{ID: "opencode-personal", Label: "Personal", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "opencode-personal", Label: "Personal", Source: "opencode-personal", FetchedAtMs: now}
			}},
			{ID: "opencode-work", Label: "Work", Collector: func(_ *string, now int64) ProviderLimits {
				return ProviderLimits{ProviderID: "opencode-work", Label: "Work", Source: "opencode-work", FetchedAtMs: now}
			}},
		},
		Only: map[string]bool{
			"grok-personal":     true,
			"grok-work":         true,
			"opencode-personal": true,
			"opencode-work":     true,
		},
	})

	if len(got) != 4 {
		t.Fatalf("profiles = %+v", got)
	}
	want := []string{"opencode-personal", "opencode-work", "grok-personal", "grok-work"}
	for i, providerID := range want {
		if got[i].ProviderID != providerID || got[i].Source != providerID {
			t.Fatalf("row %d = %+v, want %q", i, got[i], providerID)
		}
	}
}
