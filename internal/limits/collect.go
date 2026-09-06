/**
 * Facade that aggregates per-provider limits for the panel.
 *
 * Default collectors use local files/DBs. Overrides remain injectable for tests.
 */
package limits

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"

	"github.com/senna-lang/herdr-agent-usage/internal/limitscore"
	antigravityprovider "github.com/senna-lang/herdr-agent-usage/internal/providers/antigravity"
	claudeprovider "github.com/senna-lang/herdr-agent-usage/internal/providers/claude"
	"github.com/senna-lang/herdr-agent-usage/internal/providers/codex"
	"github.com/senna-lang/herdr-agent-usage/internal/providers/grok"
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

// CodexProfileCollector is one Codex account collector. Accounts may come
// from an explicit CODEX_HOME profile or OMP's observed account inventory.
// Each account displays under its own provider id.
type CodexProfileCollector = ClaudeProfileCollector

// GrokProfileCollector is one configured Grok profile's collector.
type GrokProfileCollector = ClaudeProfileCollector

// OpenCodeProfileCollector is one configured OpenCode profile's collector.
type OpenCodeProfileCollector = ClaudeProfileCollector

// AntigravityPoolCollector is one Antigravity model pool's collector. Same
// shape again, but its members are discovered from the latest statusLine
// observation rather than configured: a pool is what runs out independently,
// so it is what a row must represent.
type AntigravityPoolCollector = ClaudeProfileCollector

// CollectOptions configures CollectAllProviderLimits.
type CollectOptions struct {
	// Each profile family is collected in configuration order. Empty profile
	// slices synthesize their literal default collector for direct test callers.
	Claude      []ClaudeProfileCollector
	Codex       []CodexProfileCollector
	OpenCode    []OpenCodeProfileCollector
	Grok        []GrokProfileCollector
	Antigravity []AntigravityPoolCollector
	// Attach activity after collection (injectable for tests).
	Attach func(providers []ProviderLimits, nowMs int64) []ProviderLimits
	// Only restricts collection to these entry ids (nil = all providers).
	// Filtered providers are skipped entirely: their collectors never run.
	// ActiveProviderFilter expands a family into its entry ids, so a family
	// name never has to be matched here.
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

type codexCollectorEntry struct {
	id           string
	label        string
	accountLabel string
	collector    LimitsCollector
}

// buildCodexCollectors keeps explicitly configured CODEX_HOME profiles and
// adds every other Codex account OMP has observed. OMP rotates among OAuth
// accounts inside one harness home, so profile directories alone are not a
// complete account inventory.
func buildCodexCollectors(profiles []codex.CodexProfile, observations []AccountWindows) []CodexProfileCollector {
	observed := make([]AccountWindows, 0, len(observations))
	for _, account := range observations {
		if account.ProviderID == codex.Provider.AgentID() && account.StableIdentity() != "" {
			observed = append(observed, account)
		}
	}
	sort.Slice(observed, func(i, j int) bool {
		left, right := observedCodexAccountLabel(observed[i]), observedCodexAccountLabel(observed[j])
		if !strings.EqualFold(left, right) {
			return strings.ToLower(left) < strings.ToLower(right)
		}
		return observed[i].StableIdentity() < observed[j].StableIdentity()
	})

	matched := make(map[string]bool, len(observed))
	entries := make([]codexCollectorEntry, 0, len(profiles)+len(observed))
	for _, profile := range profiles {
		accountID := codex.AccountIDIn(profile.Home)
		var account *AccountWindows
		for i := range observed {
			if observed[i].MatchesIdentity(accountID) ||
				(accountID == "" && profile.Implicit && len(observed) == 1) {
				account = &observed[i]
				matched[observed[i].StableIdentity()] = true
				break
			}
		}

		// The zero-config profile is only a compatibility placeholder. With
		// several observed accounts and no auth identity it cannot name a real
		// third account, so do not render an extra empty row for it.
		if profile.Implicit && accountID == "" && len(observed) > 1 {
			continue
		}

		accountLabel := profile.Label
		if account != nil {
			accountLabel = observedCodexAccountLabel(*account)
		}
		profile := profile
		entries = append(entries, codexCollectorEntry{
			id:           profile.ID,
			label:        profile.Label,
			accountLabel: accountLabel,
			collector: func(_ *string, nowMs int64) ProviderLimits {
				return CollectCodexLimitsIn(profile.Home, profile.ID, profile.Label, nowMs)
			},
		})
	}

	for _, account := range observed {
		identity := account.StableIdentity()
		if matched[identity] {
			continue
		}
		account := account
		label := observedCodexAccountLabel(account)
		id := observedCodexProviderID(identity)
		entries = append(entries, codexCollectorEntry{
			id:           id,
			label:        label,
			accountLabel: label,
			collector: func(_ *string, nowMs int64) ProviderLimits {
				return BorrowedProviderLimits(account, id, label, nowMs)
			},
		})
	}

	multiAccount := len(entries) > 1
	collectors := make([]CodexProfileCollector, len(entries))
	for i, entry := range entries {
		entry := entry
		collectors[i] = CodexProfileCollector{
			ID:    entry.id,
			Label: entry.label,
			Collector: func(cwd *string, nowMs int64) ProviderLimits {
				return applyCodexGrouping(entry.collector(cwd, nowMs), entry.accountLabel, multiAccount)
			},
		}
	}
	return collectors
}

func observedCodexAccountLabel(account AccountWindows) string {
	if email := strings.TrimSpace(account.Email); email != "" {
		return email
	}
	if accountID := strings.TrimSpace(account.AccountID); accountID != "" {
		return accountID
	}
	return "account " + observedCodexProviderID(account.StableIdentity())[len("codex-observed-"):]
}

func observedCodexProviderID(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return "codex-observed-" + hex.EncodeToString(sum[:6])
}

// DefaultCollectOptions wires production local collectors (no network), one
// collector per configured profile or discovered account.
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
	codexCollectors := buildCodexCollectors(codexProfiles, limitscore.ObserveAccountWindows())

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
		Claude:      claudeCollectors,
		Codex:       codexCollectors,
		Grok:        grokCollectors,
		OpenCode:    openCodeCollectors,
		Antigravity: AntigravityPoolCollectors(),
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
}{}

// quotaFamilySpecs is the canonical list of quota-owning providers that
// expand to several panel entries: one per configured or observed account, or
// — for Antigravity — one per model pool in the latest statusLine observation.
// It is the display order, the family-id vocabulary that ui.hide_providers
// accepts, and the set billingmode.go subtracts to find the still-single
// providers, so all three cannot drift apart.
var quotaFamilySpecs = []struct {
	family, label string
	field         func(CollectOptions) []ClaudeProfileCollector
}{
	{claudeprovider.Provider.AgentID(), "Claude", func(o CollectOptions) []ClaudeProfileCollector { return o.Claude }},
	{codex.Provider.AgentID(), "Codex", func(o CollectOptions) []ClaudeProfileCollector { return o.Codex }},
	{opencode.Provider.AgentID(), "OpenCode", func(o CollectOptions) []ClaudeProfileCollector { return o.OpenCode }},
	{grok.Provider.AgentID(), "Grok", func(o CollectOptions) []ClaudeProfileCollector { return o.Grok }},
	{antigravityprovider.Provider.AgentID(), antigravityLabel, func(o CollectOptions) []ClaudeProfileCollector { return o.Antigravity }},
}

// quotaFamilyIDs is quotaFamilySpecs' id set, for the layers that only need
// to know whether a provider expands to several entries.
func quotaFamilyIDs() map[string]bool {
	ids := make(map[string]bool, len(quotaFamilySpecs))
	for _, f := range quotaFamilySpecs {
		ids[f.family] = true
	}
	return ids
}

// EntryIDs flattens the options into every entry id CollectAllProviderLimits
// would collect, in display order: each family's accounts or pools first, then
// the still-single providers. Callers that gate collection need this exact
// universe, since a gate silently missing an id would hide it.
func (o CollectOptions) EntryIDs() []string {
	entries := o.familyEntryIDs()
	out := make([]string, 0, len(entries)+len(singleCollectorQuotaSpecs))
	for _, family := range quotaFamilySpecs {
		out = append(out, entries[family.family]...)
	}
	for _, spec := range singleCollectorQuotaSpecs {
		out = append(out, spec.id)
	}
	return out
}

// defaultGatedProviderIDs is the universe for a caller with no collection
// options at hand: one id per registered quota-owning provider, which for a
// family is the family's own id.
func defaultGatedProviderIDs() []string {
	return CollectOptions{}.EntryIDs()
}

// CollectAllProviderLimits runs collectors in display order: each configured
// Claude profile (config order) -> Codex -> OpenCode -> Grok -> each
// Antigravity model pool, then attaches pane activity when configured.
// Providers excluded by opts.Only, or hidden by opts.Skip, are skipped
// (collectors never run). Pass DefaultCollectOptions() for production local
// collectors.
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
	base := make([]ProviderLimits, 0, len(quotaFamilySpecs)+len(singleCollectorQuotaSpecs))
	for _, family := range quotaFamilySpecs {
		// Each entry carries its own id so one account or pool can be hidden
		// individually, while hiding the family id ("claude") hides all of it.
		for _, spec := range withDefaultSpec(family.field(opts), family.family, family.label) {
			if opts.collects(spec.ID, family.family) {
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
