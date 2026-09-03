/**
 * Validation for the panel's hide_providers list.
 *
 * A hidden id names either one configured profile ("ai_10") or a whole
 * provider family ("grok"). Family ids come from providers.Registrations, so a
 * newly registered provider is nameable here without touching this file, and a
 * misspelled id is reported instead of silently hiding nothing.
 */
package setup

import (
	"sort"
	"strings"

	"github.com/senna-lang/herdr-agent-usage/internal/providers"
)

// normalizeProviderIDs trims, lowercases and de-duplicates configured ids,
// preserving the order they were written in.
func normalizeProviderIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.ToLower(strings.TrimSpace(raw))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// UnknownHiddenProviders returns the configured hide_providers entries that
// match neither a registered provider family nor a configured profile id, in
// configuration order. `usagebar setup` reports these: a typo would otherwise
// look exactly like a provider that legitimately has nothing to show.
func UnknownHiddenProviders(config PluginConfig) []string {
	known := knownProviderIDs(config)
	var unknown []string
	for _, id := range config.HiddenProviders {
		if !known[id] {
			unknown = append(unknown, id)
		}
	}
	return unknown
}

// KnownProviderIDs lists every id hide_providers may name, sorted, for use in
// user-facing guidance.
func KnownProviderIDs(config PluginConfig) []string {
	known := knownProviderIDs(config)
	out := make([]string, 0, len(known))
	for id := range known {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// knownProviderIDs is every registered provider family plus every configured
// profile id.
func knownProviderIDs(config PluginConfig) map[string]bool {
	known := make(map[string]bool, len(providers.All))
	for _, p := range providers.All {
		known[p.AgentID()] = true
	}
	for _, p := range config.ClaudeProfiles {
		known[strings.ToLower(p.ID)] = true
	}
	for _, p := range config.CodexProfiles {
		known[strings.ToLower(p.ID)] = true
	}
	for _, p := range config.GrokProfiles {
		known[strings.ToLower(p.ID)] = true
	}
	for _, p := range config.OpenCodeProfiles {
		known[strings.ToLower(p.ID)] = true
	}
	return known
}
