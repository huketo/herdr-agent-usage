/**
 * Per-pane context resolution for the panel's context-only section.
 *
 * The panel's aggregation layer stays free of herdr and of the provider
 * adapters; this is the injected implementation that reaches both. It re-reads
 * the pane rather than trusting the snapshot because the snapshot keeps only
 * the session's value, and a provider needs the session's kind to interpret it
 * (an id names a conversation, a path names a transcript). Resolving through
 * the same GetPaneInfo + ResolveUsage pair the sidebar uses also guarantees the
 * panel and the pane's own $context row never disagree.
 */
package update

import (
	"github.com/senna-lang/herdr-agent-usage/internal/core"
	"github.com/senna-lang/herdr-agent-usage/internal/herdrcli"
	"github.com/senna-lang/herdr-agent-usage/internal/limits"
	"github.com/senna-lang/herdr-agent-usage/internal/provider"
	"github.com/senna-lang/herdr-agent-usage/internal/providers"
)

// ResolveContextUsageForPane resolves one open pane's context occupancy
// through its registered provider. nil means the occupancy is unknown, which
// includes a pane whose agent changed since the snapshot was taken: reporting
// the previous agent's session would attribute one agent's context to another.
func ResolveContextUsageForPane(pane limits.OpenPaneSnapshot) *core.ContextUsage {
	if pane.PaneID == "" || pane.Agent == "" {
		return nil
	}
	info := herdrcli.GetPaneInfo(pane.PaneID)
	if info.Agent == nil || *info.Agent != pane.Agent {
		return nil
	}
	p := providers.FindProvider(*info.Agent)
	if p == nil {
		return nil
	}
	paneID := pane.PaneID
	return p.ResolveUsage(provider.UsageResolveInput{
		Session: info.AgentSession,
		Cwd:     paneCwdForUpdate(info),
		PaneID:  &paneID,
	})
}
