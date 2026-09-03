/**
 * Context-only panes: the panel section for providers that report context
 * occupancy but own no quota.
 *
 * A provider registered CapContextOnly (Cursor, Antigravity) has no window to
 * run out of, so it never produces a ProviderLimits block. Its panes would
 * otherwise be invisible in the panel even while they are the pane the user is
 * working in. This section states what is actually known about them: how full
 * the context is, per open pane.
 *
 * The eligible agent set is derived from providers.Registrations rather than
 * listed here, so registering another context-only provider surfaces it in the
 * panel with no change to this file. Resolution itself is injected: reading a
 * pane's occupancy needs herdr and the provider adapters, which belong to the
 * caller's layer, not to this aggregation.
 */
package limits

import (
	"sort"

	"github.com/senna-lang/herdr-agent-usage/internal/core"
	"github.com/senna-lang/herdr-agent-usage/internal/providers"
)

// ContextPaneUsage is one open pane of a context-only provider.
type ContextPaneUsage struct {
	PaneID string
	// Agent is the pane's agent id, which is also the provider id for these
	// providers: they have no accounts or profiles to tell apart.
	Agent string
	Label string
	Usage core.ContextUsage
}

// ContextPaneDeps injects per-pane context resolution.
type ContextPaneDeps struct {
	// ResolveUsage returns the pane's context occupancy, or nil when it cannot
	// be determined (no session yet, unreadable state, foreign version).
	ResolveUsage func(pane OpenPaneSnapshot) *core.ContextUsage
}

// CollectContextPaneUsage returns one entry per open pane whose provider is
// registered CapContextOnly and whose occupancy could be resolved. Panes that
// resolve to nothing are dropped rather than shown as zero: the panel must not
// claim an empty context for a session it simply could not read.
//
// Entries are ordered by provider registration order, then by pane label and
// id, so a redraw does not reshuffle rows.
func CollectContextPaneUsage(openPanes []OpenPaneSnapshot, deps ContextPaneDeps) []ContextPaneUsage {
	if deps.ResolveUsage == nil || len(openPanes) == 0 {
		return nil
	}

	rank := contextOnlyProviderRank()
	if len(rank) == 0 {
		return nil
	}

	out := make([]ContextPaneUsage, 0, len(openPanes))
	for _, pane := range openPanes {
		if _, ok := rank[pane.Agent]; !ok {
			continue
		}
		usage := deps.ResolveUsage(pane)
		if usage == nil {
			continue
		}
		out = append(out, ContextPaneUsage{
			PaneID: pane.PaneID,
			Agent:  pane.Agent,
			Label:  pane.Label,
			Usage:  *usage,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if rank[out[i].Agent] != rank[out[j].Agent] {
			return rank[out[i].Agent] < rank[out[j].Agent]
		}
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].PaneID < out[j].PaneID
	})
	return out
}

// contextOnlyProviderRank maps each context-only provider id to its
// registration position, which doubles as the section's display order.
func contextOnlyProviderRank() map[string]int {
	ids := providers.IDsWithCapability(providers.CapContextOnly)
	rank := make(map[string]int, len(ids))
	for i, id := range ids {
		rank[id] = i
	}
	return rank
}

// HiddenProviderSet turns configured provider ids into a lookup set for
// CollectOptions.Skip and for filtering the context section.
func HiddenProviderSet(ids []string) map[string]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// FilterHiddenContextPanes drops the rows of providers the user hid. These
// providers have no profiles, so their agent id is the only id to match.
func FilterHiddenContextPanes(panes []ContextPaneUsage, hidden map[string]bool) []ContextPaneUsage {
	if len(hidden) == 0 || len(panes) == 0 {
		return panes
	}
	out := make([]ContextPaneUsage, 0, len(panes))
	for _, p := range panes {
		if hidden[p.Agent] {
			continue
		}
		out = append(out, p)
	}
	return out
}
