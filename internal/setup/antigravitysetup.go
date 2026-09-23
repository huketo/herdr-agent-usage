/**
 * Antigravity CLI statusLine setup guidance.
 *
 * Antigravity's context and quota usage reach this plugin only through its
 * statusLine, configured in ~/.gemini/antigravity-cli/settings.json (see
 * https://antigravity.google/docs/cli/statusline). Setup deliberately prints
 * instructions and never edits that file: it is Antigravity's own settings
 * file, and a user may already have a statusLine command there.
 *
 * The snippet sets `stack_with_default`, which renders this plugin's line
 * below Antigravity's built-in status line instead of replacing it, so the
 * user keeps agy's own model, quota, and mode indicators.
 */
package setup

// AntigravityStatusLineSnippet is the settings.json entry enabling Antigravity
// context and quota usage, given the plugin root.
func AntigravityStatusLineSnippet(pluginRoot string) string {
	return `  "statusLine": {
    "type": "command",
    "command": "bash ` + pluginRoot + `/bin/run-antigravity-statusline.sh",
    "stack_with_default": true
  }`
}

// antigravitySetupLines renders the Antigravity section of the setup report.
func antigravitySetupLines(pluginRoot string) []string {
	return []string{
		"Antigravity CLI statusLine (optional, for Antigravity context + 5h/weekly quota):",
		"  Add to ~/.gemini/antigravity-cli/settings.json — this plugin never edits that file.",
		"  `stack_with_default` keeps agy's built-in status line and adds this one below it.",
		"",
		AntigravityStatusLineSnippet(pluginRoot),
		"",
		"  If a statusLine command is already configured, keep it and chain:",
		"  have your existing script pass its stdin through to",
		"  " + pluginRoot + "/bin/run-antigravity-statusline.sh and print both outputs.",
		"",
	}
}
