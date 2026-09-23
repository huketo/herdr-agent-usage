# Antigravity CLI contract

The tested baseline for Antigravity CLI (`agy`) support, recorded for drift
monitoring.

## Tested version

| | |
| --- | --- |
| Antigravity CLI | `1.2.8` (Linux x64, WSL2); earlier baseline `1.2.3` (Homebrew cask `antigravity-cli`, macOS) |
| Verified on | `~/.gemini/antigravity-cli` default app-data directory |

## Where the contract is documented

Antigravity documents the statusLine publicly at
[antigravity.google/docs/cli/statusline](https://antigravity.google/docs/cli/statusline)
(configuration block and payload fields) and
[/docs/cli/commands/statusline](https://antigravity.google/docs/cli/commands/statusline)
(the `/statusline` slash command). The contract drift checker watches the
first page (`scripts/contractdrift/provider-contracts.json`, id
`antigravity`). The published payload example is sanitized and older than the
tested version (it shows `1.0.13`), so the captured-payload fixtures in
`internal/providers/antigravity/statusline_test.go` and
`statuslinecmd_test.go` remain the exact baseline and must be re-captured
when the tested version advances.

Configuration lives in `~/.gemini/antigravity-cli/settings.json`:

```json
"statusLine": {
  "type": "command",
  "command": "bash <plugin-root>/bin/run-antigravity-statusline.sh",
  "stack_with_default": true
}
```

`stack_with_default` (agy `1.0.6`+) renders the command's output below the
built-in status line instead of replacing it. `usagebar setup` prints this
block; the plugin never edits Antigravity's settings file. `/statusline
<command>` inside `agy` writes the same `command` key.

Antigravity ships an `agy 1.2.2`-era `history.jsonl` / `presence/` layout in
some installs and a `conversation_summaries.db` / `cache/conversation_metadata.json`
layout in others (observed switching between a same-day reinstall at `1.2.3`).
This plugin depends on neither: see "Not used" below.

## Payload semantics relied upon

From the statusLine payload, this provider reads:

- `conversation_id` — the session identity. herdr's `antigravity_cli`
  integration reports this as `agent_session` with **`kind: "id"`**, not
  `"path"` — confirmed against a real `herdr pane get` while `agy` ran inside
  a herdr pane. This is the only currently-registered provider using `kind:
  "id"` rather than a session file path. Before the first turn agy sends
  payloads with an **empty** `conversation_id` (observed on `1.2.8` while the
  CLI was still `initializing`) that already carry the account's `quota`; see
  "Quota semantics".
- `context_window.total_input_tokens` — a running, non-nullable integer.
  Unlike Cursor, Antigravity never nulls this field out early in a session; a
  captured `0` is a real "no turns yet" observation, not a missing one.
- `context_window.context_window_size` — the active window, reported directly
  (no static model→window table is needed).
- `context_window.current_usage.{input_tokens,cache_creation_input_tokens,cache_read_input_tokens}`
  — the latest completed turn's cache breakdown, present only after the first
  turn. Feeds `$cache_*`; there is no session-cumulative cache counter
  locally, so `SessionCache` is intentionally left unset.
- `quota` — a flat map keyed by Antigravity's own bucket id,
  `<pool>-<window>`. `1.2.8` reports four buckets: `gemini-5h` and
  `gemini-weekly` (Antigravity's native models), `3p-5h` and `3p-weekly`
  (third-party models — Claude, GPT-OSS — routed through it). The `1.2.3`
  capture had only the two weekly buckets. Each carries `remaining_fraction`
  (0-1) and an absolute `reset_time` (RFC3339). `reset_in_seconds` is also
  present but not used: it is only accurate at capture time. The pool names
  match the groups `agy -p /quota --output-format json` prints ("Gemini
  Models", "Claude and GPT models"); the panel shows them as "Gemini" and
  "Claude & GPT".
- `plan_tier` — a human label for the account's plan (e.g. `"Antigravity
  Starter Quota"`), shown as `$provider`'s plan type.
- `transcript_path` — recorded by herdr's hook as `agent_session_path`
  alongside `agent_session_id`; not currently read by this provider, since the
  statusLine payload already carries everything needed.

## Not used

- **`~/.gemini/antigravity-cli/conversations/*.db`** — one SQLite file per
  conversation, with `gen_metadata`/`steps`/`executor_metadata` payload
  columns that are protobuf-encoded. Whether per-turn token counts live in
  there was never confirmed, and is moot: the statusLine payload already
  reports the same figures directly, in a stable JSON shape, with no schema
  reverse-engineering or SQLite dependency required.
- **`history.jsonl` / `presence/*.lock`** — used by some installs to map a
  conversation id to a workspace path; unneeded because herdr's own
  `agent_session` already carries the conversation id directly (`kind:
  "id"`), and the statusLine payload carries it a second time.

## Identity semantics

Antigravity mints a new conversation id when a session is cleared or resumed
into a new conversation, while herdr keeps reporting the one observed when the
pane launched — the same failure mode Cursor's provider documents
(herdrdev/herdr#2510) and handles the same way: snapshots are stored by
conversation id but also record the herdr pane id, and resolution falls back
to pane identity when the reported id's snapshot is stale or has been
superseded by a newer one on the same pane. Working directory alone is not
sufficient: two Antigravity panes may share one repository.

## Quota semantics

Unlike Cursor, Antigravity's statusLine reports account-wide quota directly,
so it is registered `CapOwnsSubscriptionQuota` rather than `CapContextOnly`.
Quota is billed per Google account, not per conversation, so the bridge keeps
it in its own snapshot (`herdr-usagebar/account.json`, beside the sessions
directory) that every payload reporting quota refreshes, including pre-turn
payloads with no conversation id. The limits collector reads that snapshot
while it is fresh (four hours), whichever pane wrote it.

Each pool is one panel entry, the way the other harnesses expand to one entry
per account: the 5-hour window is Primary and the weekly window Secondary.
The native pool keeps the bare `agy` id, so a pane's sidebar `$limit` shows
the Gemini pool; the third-party pool is `agy-3p`. Pools are derived from the
bucket ids, so a pool Antigravity adds appears under its raw id, and a window
name other than `5h`/`weekly` is skipped rather than guessed into a bar.

## Context percentage

The rendered percentage is computed from `total_input_tokens` over
`context_window_size`. In the `1.2.3` capture this equals agy's own
`used_percentage` (18558 / 1048576 = 1.7698%). The published example instead
matches (input + output) / size (149318 / 1048576 = 14.24%), so if a future
capture disagrees, re-check which total agy's own meter uses.
