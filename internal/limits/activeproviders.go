/**
 * Derives the set of provider ids that have at least one open agent pane, so
 * the limits panel can hide providers not running anywhere in Herdr.
 *
 * A pane names a family ("claude", "agy"), while the panel collects entries:
 * one per configured account, or per allowance pool the running CLI reports.
 * One open pane therefore activates every entry of its family — opening one
 * Claude pane is meant to show every configured Claude account side by side —
 * so activation expands the family through the very collectors this pass will
 * run, rather than a second list of ids kept in step by hand.
 */
package limits

import "strings"

// ActiveProviderFilter builds the CollectOptions.Only filter from a pane query
// result, expanding families through opts' own collectors. When the query
// failed (paneQueryOK=false) it returns nil — fail-open to all providers
// rather than blanking the panel on a transient herdr error. Only a confirmed
// pane list may hide providers.
func ActiveProviderFilter(openPanes []OpenPaneSnapshot, paneQueryOK bool, opts CollectOptions) map[string]bool {
	if !paneQueryOK {
		return nil
	}
	return ActiveProviderSet(openPanes, opts)
}

// ActiveProviderSet returns the provider ids that have at least one open pane.
// Agent ids match case-insensitively; unknown agents are ignored. The result
// is never nil: an empty set means "no agent panes open".
func ActiveProviderSet(openPanes []OpenPaneSnapshot, opts CollectOptions) map[string]bool {
	profiles := ResolvedClaudeProfiles()
	codexProfiles := ResolvedCodexProfiles()
	grokProfiles := ResolvedGrokProfiles()
	openCodeProfiles := ResolvedOpenCodeProfiles()
	return activeProviderSetWith(openPanes,
		BuildPaneActivityProviderResolver(profiles, codexProfiles, grokProfiles, openCodeProfiles),
		opts.familyEntryIDs())
}

// familyEntryIDs lists the entry ids each family expands to in these options,
// which is exactly what CollectAllProviderLimits will iterate.
func (o CollectOptions) familyEntryIDs() map[string][]string {
	out := make(map[string][]string, len(quotaFamilySpecs))
	for _, family := range quotaFamilySpecs {
		specs := withDefaultSpec(family.field(o), family.family, family.label)
		ids := make([]string, 0, len(specs))
		for _, spec := range specs {
			ids = append(ids, spec.ID)
		}
		out[family.family] = ids
	}
	return out
}

// familyForID maps a pane's agent id, or a provider id resolved from its
// session, onto the family whose entries it belongs to. The family's own id
// always answers for itself, which covers a harness with no configured
// accounts.
func familyForID(entryIDs map[string][]string, id string) (string, bool) {
	if _, ok := entryIDs[id]; ok {
		return id, true
	}
	for family, ids := range entryIDs {
		for _, entryID := range ids {
			if entryID == id {
				return family, true
			}
		}
	}
	return "", false
}

func activeProviderSetWith(openPanes []OpenPaneSnapshot, resolve func(OpenPaneSnapshot) (string, bool), entryIDs map[string][]string) map[string]bool {
	set := make(map[string]bool)
	activate := func(family string) {
		for _, id := range entryIDs[family] {
			set[id] = true
		}
	}

	for _, pane := range openPanes {
		// A harness pane names its family directly; anything else (an OMP or
		// Pi pane routed through someone's collector) must be resolved first.
		if family, ok := familyForID(entryIDs, strings.ToLower(pane.Agent)); ok {
			activate(family)
			continue
		}
		providerID, ok := resolve(pane)
		if !ok {
			continue
		}
		if family, ok := familyForID(entryIDs, providerID); ok {
			activate(family)
			continue
		}
		for _, supportedID := range singleCollectorProviderIDs {
			if providerID == supportedID {
				set[providerID] = true
				break
			}
		}
	}
	return set
}
