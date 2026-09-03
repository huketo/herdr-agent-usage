/**
 * Facade that aggregates per-provider limits for the panel.
 *
 * Default collectors use local files/DBs. Overrides remain injectable for tests.
 */
package limits

import (
	"path/filepath"

	"github.com/senna-lang/herdr-agent-usage/internal/providers/antigravity"
	"github.com/senna-lang/herdr-agent-usage/internal/providers/opencode"
)

// LimitsCollector fetches one provider's rate-limit snapshot.
type LimitsCollector func(cwd *string, nowMs int64) ProviderLimits

// ClaudeProfileCollector is one configured Claude profile's collector, keyed by
// that profile's own provider id/label so multiple accounts collect and display
// independently instead of sharing the single literal "claude" id.
type ClaudeProfileCollector struct {
	ID        string
	Label     string
	Collector LimitsCollector
}

// CodexProfileCollector is one configured Codex profile's collector. Same
// shape as ClaudeProfileCollector: each account collects and displays under
// its own provider id instead of sharing the single literal "codex" id.
type CodexProfileCollector = ClaudeProfileCollector

// GrokProfileCollector is one configured Grok profile's collector.
type GrokProfileCollector = ClaudeProfileCollector

// OpenCodeProfileCollector is one configured OpenCode profile's collector.
type OpenCodeProfileCollector = ClaudeProfileCollector

// CollectOptions configures CollectAllProviderLimits.
type CollectOptions struct {
	// Each profile family is collected in configuration order. Empty profile
	// slices synthesize their literal default collector for direct test callers.
	Claude   []ClaudeProfileCollector
	Codex    []CodexProfileCollector
	OpenCode []OpenCodeProfileCollector
	Grok     []GrokProfileCollector
	// Antigravity has no per-account profile concept (one Google account per
	// install), so unlike the profile families above it is a single
	// injectable collector rather than a slice. Nil leaves it uncollected in
	// a bare CollectOptions{}, matching the profile families' unconfigured
	// stub behavior for direct test callers.
	Antigravity LimitsCollector
	// Attach activity after collection (injectable for tests).
	Attach func(providers []ProviderLimits, nowMs int64) []ProviderLimits
	// Only restricts collection to these provider ids (nil = all providers).
	// Filtered providers are skipped entirely: their collectors never run.
	Only map[string]bool
	// Skip hides provider ids the user configured out of the panel, by profile
	// id or by family id. It is applied after Only, so a hidden provider stays
	// hidden even when something else would have shown it — including the
	// panel's own "show every provider" mode.
	Skip map[string]bool
}

// collects reports whether one collector should run. providerID is the
// profile's own id; family is the provider family it belongs to, which is what
// a user names when hiding a whole harness rather than one account.
func (o CollectOptions) collects(providerID, family string) bool {
	if o.Only != nil && !o.Only[providerID] {
		return false
	}
	return !o.Skip[providerID] && !o.Skip[family]
}

// withDefaultSpec returns specs, or the family's literal default collector
// when none are configured, which keeps direct test callers working.
func withDefaultSpec(specs []ClaudeProfileCollector, id, label string) []ClaudeProfileCollector {
	if len(specs) > 0 {
		return specs
	}
	return []ClaudeProfileCollector{{ID: id, Label: label}}
}

// DefaultCollectOptions wires production local collectors (no network), one
// collector per configured Claude, Codex, Grok, or OpenCode profile.
func DefaultCollectOptions() CollectOptions {
	profiles := ResolvedClaudeProfiles()
	multiProfile := len(profiles) > 1
	claudeCollectors := make([]ClaudeProfileCollector, len(profiles))
	for i, profile := range profiles {
		claudeCollectors[i] = ClaudeProfileCollector{
			ID:    profile.ID,
			Label: profile.Label,
			Collector: func(_ *string, nowMs int64) ProviderLimits {
				pl := CollectClaudeLimits(nowMs, CollectClaudeLimitsOptions{
					StatusLineCachePath: profile.LimitsCache,
					ClaudeJSONPath:      profile.JSONPath,
				})
				pl.ProviderID = profile.ID
				pl.Label = profile.Label
				// When 2+ accounts are configured, every row nests under one
				// shared "Claude" group in the panel, labeled by its real
				// logged-in email rather than the profile's own label — so
				// the account behind each row is always verifiable.
				return applyProfileGrouping(pl, profile, multiProfile)
			},
		}
	}

	codexProfiles := ResolvedCodexProfiles()
	multiCodex := len(codexProfiles) > 1
	codexCollectors := make([]CodexProfileCollector, len(codexProfiles))
	for i, profile := range codexProfiles {
		codexCollectors[i] = CodexProfileCollector{
			ID:    profile.ID,
			Label: profile.Label,
			Collector: func(_ *string, nowMs int64) ProviderLimits {
				pl := CollectCodexLimitsIn(profile.Home, profile.ID, profile.Label, nowMs)
				return applyCodexProfileGrouping(pl, profile, multiCodex)
			},
		}
	}

	grokProfiles := ResolvedGrokProfiles()
	multiGrok := len(grokProfiles) > 1
	grokCollectors := make([]GrokProfileCollector, len(grokProfiles))
	for i, profile := range grokProfiles {
		grokCollectors[i] = GrokProfileCollector{
			ID:    profile.ID,
			Label: profile.Label,
			Collector: func(_ *string, nowMs int64) ProviderLimits {
				authPath := ""
				if !profile.Implicit {
					authPath = filepath.Join(profile.Home, "auth.json")
				}
				pl := CollectGrokLimits(nowMs, CollectGrokLimitsOptions{AuthPath: authPath})
				pl.ProviderID = profile.ID
				pl.Label = profile.Label
				return applyGrokProfileGrouping(pl, profile, multiGrok)
			},
		}
	}

	openCodeProfiles := ResolvedOpenCodeProfiles()
	multiOpenCode := len(openCodeProfiles) > 1
	openCodeCollectors := make([]OpenCodeProfileCollector, len(openCodeProfiles))
	for i, profile := range openCodeProfiles {
		openCodeCollectors[i] = OpenCodeProfileCollector{
			ID:    profile.ID,
			Label: profile.Label,
			Collector: func(_ *string, nowMs int64) ProviderLimits {
				dbPath := ""
				if !profile.Implicit {
					dbPath = opencode.ResolveOpenCodeDBPathIn(profile.DataDir)
				}
				pl := CollectOpenCodeLimits(nowMs, dbPath)
				pl.ProviderID = profile.ID
				pl.Label = profile.Label
				return applyOpenCodeProfileGrouping(pl, profile, multiOpenCode)
			},
		}
	}

	return CollectOptions{
		Claude:   claudeCollectors,
		Codex:    codexCollectors,
		Grok:     grokCollectors,
		OpenCode: openCodeCollectors,
		Antigravity: func(_ *string, nowMs int64) ProviderLimits {
			return CollectAntigravityLimits(nowMs, CollectAntigravityLimitsOptions{})
		},
	}
}

// singleCollectorQuotaSpecs pairs each still-single quota-owning provider
// id/label with the CollectOptions field carrying its collector. Claude and
// Codex are omitted because they expand to one collector per configured
// profile. This list's id set is checked against
// providers.IDsWithCapability(CapOwnsSubscriptionQuota) minus those profile
// families by TestSingleCollectorQuotaSpecs_MatchCapabilityRegistrations, so a
// newly registered quota-owning provider that isn't wired here fails that
// test instead of silently never appearing in the panel.
var singleCollectorQuotaSpecs = []struct {
	id, label string
	field     func(CollectOptions) LimitsCollector
}{
	{
		id:    antigravity.Provider.AgentID(),
		label: "Antigravity",
		field: func(o CollectOptions) LimitsCollector { return o.Antigravity },
	},
}

// CollectAllProviderLimits runs collectors in display order: each configured
// Claude profile (config order) -> Codex -> OpenCode -> Grok, then attaches
// pane activity when configured. Providers excluded by opts.Only, or hidden by
// opts.Skip, are skipped (collectors never run). Pass DefaultCollectOptions()
// for production local collectors.
func CollectAllProviderLimits(cwd *string, nowMs int64, opts CollectOptions) []ProviderLimits {
	collect := func(collector LimitsCollector, id, label string) ProviderLimits {
		if collector != nil {
			return collector(cwd, nowMs)
		}
		return ProviderLimits{
			ProviderID:  id,
			Label:       label,
			Source:      "stub",
			FetchedAtMs: nowMs,
			Note:        strPtr("limits collector not configured"),
		}
	}

	// Collection order is the panel's display order. Each family carries its
	// own id so a profile id ("ai_10") can be hidden individually while
	// hiding the family id ("claude") hides every one of its accounts.
	families := []struct {
		family string
		specs  []ClaudeProfileCollector
	}{
		{"claude", withDefaultSpec(opts.Claude, "claude", "Claude")},
		{"codex", withDefaultSpec(opts.Codex, "codex", "Codex")},
		{"opencode", withDefaultSpec(opts.OpenCode, "opencode", "OpenCode")},
		{"grok", withDefaultSpec(opts.Grok, "grok", "Grok")},
	}

	base := make([]ProviderLimits, 0, len(singleCollectorQuotaSpecs))
	for _, f := range families {
		for _, spec := range f.specs {
			if opts.collects(spec.ID, f.family) {
				base = append(base, collect(spec.Collector, spec.ID, spec.Label))
			}
		}
	}
	for _, spec := range singleCollectorQuotaSpecs {
		if opts.collects(spec.id, spec.id) {
			base = append(base, collect(spec.field(opts), spec.id, spec.label))
		}
	}

	if opts.Attach != nil {
		return opts.Attach(base, nowMs)
	}
	return base
}

func strPtr(s string) *string { return &s }
