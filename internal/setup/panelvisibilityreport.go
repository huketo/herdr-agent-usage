/**
 * setup report lines for the panel's provider visibility settings.
 */
package setup

import "strings"

// panelVisibilityReportLines reports how the panel decides which providers to
// show, and flags a hide_providers entry that names nothing: a typo would
// otherwise be indistinguishable from a provider that simply has no data.
func panelVisibilityReportLines(config PluginConfig) []string {
	visibility := "providers with an open agent pane"
	if config.ShowAllProviders {
		visibility = "every configured provider"
	}
	lines := []string{"  panel shows " + visibility}

	if len(config.HiddenProviders) > 0 {
		lines = append(lines, "  hidden: "+strings.Join(config.HiddenProviders, ", "))
	}
	if unknown := UnknownHiddenProviders(config); len(unknown) > 0 {
		lines = append(lines,
			"  ! hide_providers names no provider or profile: "+strings.Join(unknown, ", "),
			"    known ids: "+strings.Join(KnownProviderIDs(config), ", "))
	}
	return lines
}
