/**
 * Tests for the panel visibility settings: parsing, normalization, and the
 * setup report that catches an id naming nothing.
 */
package setup

import (
	"strings"
	"testing"

	"github.com/senna-lang/herdr-agent-usage/internal/providers/claude"
)

func TestParsePluginConfigTOML_PanelVisibility(t *testing.T) {
	cfg := ParsePluginConfigTOML(`
[ui]
show_all_providers = true
hide_providers = ["opencode", "GROK", " grok ", ""]
`)

	if !cfg.ShowAllProviders {
		t.Fatal("show_all_providers must be honored")
	}
	if len(cfg.HiddenProviders) != 2 || cfg.HiddenProviders[0] != "opencode" || cfg.HiddenProviders[1] != "grok" {
		t.Fatalf("HiddenProviders = %#v, want normalized [opencode grok]", cfg.HiddenProviders)
	}
}

// The panel default must stay "only providers with an open pane" for a config
// that predates these keys.
func TestParsePluginConfigTOML_PanelVisibilityDefaults(t *testing.T) {
	cfg := ParsePluginConfigTOML("[ui]\nlimit_percent = \"used\"\n")

	if cfg.ShowAllProviders {
		t.Fatal("show_all_providers must default to false")
	}
	if cfg.HiddenProviders != nil {
		t.Fatalf("HiddenProviders = %#v, want nil", cfg.HiddenProviders)
	}
}

// The seeded body must round-trip, or `usagebar setup` would write a file it
// cannot read back.
func TestDefaultPluginConfigTOML_RoundTripsPanelVisibility(t *testing.T) {
	config := DefaultPluginConfig
	config.ShowAllProviders = true
	config.HiddenProviders = []string{"opencode", "grok"}

	body := DefaultPluginConfigTOML(config)
	parsed := ParsePluginConfigTOML(body)

	if !parsed.ShowAllProviders {
		t.Fatalf("show_all_providers lost in:\n%s", body)
	}
	if strings.Join(parsed.HiddenProviders, ",") != "opencode,grok" {
		t.Fatalf("HiddenProviders = %#v in:\n%s", parsed.HiddenProviders, body)
	}
}

// A family id and a configured profile id are both valid; anything else is
// reported rather than silently hiding nothing.
func TestUnknownHiddenProviders(t *testing.T) {
	config := PluginConfig{
		HiddenProviders: []string{"grok", "ai_10", "gpt"},
		ClaudeProfiles:  []claude.ProfileSpec{{ID: "ai_10", ConfigDir: "~/.claude"}},
	}

	unknown := UnknownHiddenProviders(config)
	if len(unknown) != 1 || unknown[0] != "gpt" {
		t.Fatalf("unknown = %#v, want [gpt]", unknown)
	}
	if known := KnownProviderIDs(config); len(known) == 0 {
		t.Fatal("known ids must be listable for the setup report")
	}
}

func TestPanelVisibilityReportLines(t *testing.T) {
	quiet := panelVisibilityReportLines(PluginConfig{})
	if !strings.Contains(strings.Join(quiet, "\n"), "open agent pane") {
		t.Fatalf("default visibility must be reported: %#v", quiet)
	}

	noisy := panelVisibilityReportLines(PluginConfig{
		ShowAllProviders: true,
		HiddenProviders:  []string{"grok", "nope"},
	})
	joined := strings.Join(noisy, "\n")
	for _, want := range []string{"every configured provider", "hidden: grok, nope", "! hide_providers names no provider"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
}
