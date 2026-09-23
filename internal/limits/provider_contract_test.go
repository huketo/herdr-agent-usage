/**
 * Exhaustiveness contract between providers.Registrations and the
 * internal/limits dispatch tables that depend on it.
 *
 * Each test below asserts bidirectional equality (declared set == table's
 * key set) so both a missing addition and a stale leftover are caught. See
 * AGENTS.md's "Exhaustive Constraints for Evolving Definitions": a provider
 * newly declaring a capability must fail one of these tests until every
 * dependent table is updated, instead of compiling clean and silently never
 * appearing in the panel.
 */
package limits

import (
	"sort"
	"testing"

	"github.com/senna-lang/herdr-agent-usage/internal/providers"
)

func sortedCopy(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

func assertSameIDSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	gotSorted, wantSorted := sortedCopy(got), sortedCopy(want)
	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("%s: got %v, want %v", label, gotSorted, wantSorted)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("%s: got %v, want %v", label, gotSorted, wantSorted)
		}
	}
}

// TestAgentToProvider_MatchesRegistrations guards attachactivity.go's derived
// map: every registered provider must resolve pane activity by its own id.
func TestAgentToProvider_MatchesRegistrations(t *testing.T) {
	var want []string
	for _, p := range providers.All {
		want = append(want, p.AgentID())
	}
	var got []string
	for agentID, providerID := range agentToProvider {
		if agentID != providerID {
			t.Errorf("agentToProvider[%q] = %q, want identity", agentID, providerID)
		}
		got = append(got, agentID)
	}
	assertSameIDSet(t, "agentToProvider keys", got, want)
}

// singleCollectorQuotaOwnerWant is every quota-owning provider that is still a
// single collector: the capability registrations minus the families that
// expand to one entry per account or pool.
func singleCollectorQuotaOwnerWant() []string {
	families := quotaFamilyIDs()
	var want []string
	for _, id := range providers.IDsWithCapability(providers.CapOwnsSubscriptionQuota) {
		if !families[id] {
			want = append(want, id)
		}
	}
	return want
}

// TestQuotaFamilySpecs_AreRegisteredQuotaOwners guards collect.go's family
// table: a family id must name a provider that declares it owns quota, so a
// renamed or unregistered provider cannot linger as a phantom panel section.
func TestQuotaFamilySpecs_AreRegisteredQuotaOwners(t *testing.T) {
	owners := map[string]bool{}
	for _, id := range providers.IDsWithCapability(providers.CapOwnsSubscriptionQuota) {
		owners[id] = true
	}
	for _, spec := range quotaFamilySpecs {
		if !owners[spec.family] {
			t.Fatalf("quotaFamilySpecs has %q, which owns no subscription quota", spec.family)
		}
		if spec.label == "" {
			t.Fatalf("quotaFamilySpecs %q has no label", spec.family)
		}
	}
}

// TestSingleCollectorQuotaOwnerIDs_MatchCapabilityRegistrations guards
// billingmode.go's derived list: every still-single quota-owning provider must
// be present, and no family must be.
func TestSingleCollectorQuotaOwnerIDs_MatchCapabilityRegistrations(t *testing.T) {
	assertSameIDSet(t, "singleCollectorProviderIDs", singleCollectorProviderIDs, singleCollectorQuotaOwnerWant())
}

// TestSingleCollectorQuotaSpecs_MatchCapabilityRegistrations guards collect.go's
// CollectAllProviderLimits wiring: every quota-owning provider that is still
// a single collector must have a collect spec.
func TestSingleCollectorQuotaSpecs_MatchCapabilityRegistrations(t *testing.T) {
	var got []string
	for _, s := range singleCollectorQuotaSpecs {
		got = append(got, s.id)
	}
	assertSameIDSet(t, "singleCollectorQuotaSpecs ids", got, singleCollectorQuotaOwnerWant())
}

// TestLimitIDSlotTables_MatchCapabilityRegistrations guards windowpool.go's
// per-provider limit-id vocabulary: every quota-owning provider (including
// Claude) must have a table, and no other provider must have one.
func TestLimitIDSlotTables_MatchCapabilityRegistrations(t *testing.T) {
	want := providers.IDsWithCapability(providers.CapOwnsSubscriptionQuota)
	var got []string
	for id := range limitIDSlotTables {
		got = append(got, id)
	}
	assertSameIDSet(t, "limitIDSlotTables keys", got, want)
}

// TestContextOnlyProviders_AppearInNoLimitsTable guards the other direction of
// the capability partition: a provider that reports context only must not
// appear in any limits dispatch table, so it can never contribute a limit
// window, a panel block, or a rate-limit notification.
func TestContextOnlyProviders_AppearInNoLimitsTable(t *testing.T) {
	contextOnly := map[string]bool{}
	for _, id := range providers.IDsWithCapability(providers.CapContextOnly) {
		contextOnly[id] = true
	}
	if len(contextOnly) == 0 {
		t.Skip("no context-only providers registered")
	}
	for _, id := range singleCollectorProviderIDs {
		if contextOnly[id] {
			t.Errorf("%s: context-only provider present in singleCollectorProviderIDs", id)
		}
	}
	for _, spec := range singleCollectorQuotaSpecs {
		if contextOnly[spec.id] {
			t.Errorf("%s: context-only provider present in singleCollectorQuotaSpecs", spec.id)
		}
	}
	for id := range limitIDSlotTables {
		if contextOnly[id] {
			t.Errorf("%s: context-only provider present in limitIDSlotTables", id)
		}
	}
}
