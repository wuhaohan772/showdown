# Persistent Opponent Session — Design

Date: 2026-07-04
Status: approved

## Purpose

P2 of the roadmap. Today every agent decision is a fresh `claude -p`
spawn sending the full ~2.4k-token prompt: prompt caching never hits
(the CLI's cache breakpoint is the end of the single user message and
matching is exact), so sonnet costs ~$0.015/decision and every call
pays the 2-5s process spawn. Verified fix: one long-lived
`claude --input-format stream-json --output-format stream-json`
process per match — conversation history becomes a cached prefix
(measured: turn 2 cache_read 2069, ~$0.0012) and spawn cost is paid
once. Expected: ~$0.001-0.003/decision, decisions bounded by decode
time only. Match-end reaction rides the same session.

Out of scope (deliberate): ReactionPrompt project-nudge (session
context may fix it for free; revisit after), conversation-length caps
or mid-match session restarts for cost (matches end in 10-30 hands;
growth is pennies), codex/gemini sessions.

## Session core — `internal/agent/session.go`

`Session` owns one CLI process for a whole match:

```
claude -p --verbose --input-format stream-json --output-format stream-json
       --disallowedTools '*' --system-prompt <claudeSystemPrompt static>
       [--model <m>]
```

cwd = launch dir (ADR-0002), env `MAX_THINKING_TOKENS=0` (plus
`os.Environ()`), same as the stateless adapter.

- `Ask(ctx, prompt string) (Response, error)` — the exact `Asker`
  signature. Writes one JSONL line to stdin:
  `{"type":"user","message":{"role":"user","content":[{"type":"text","text":<prompt>}]}}`
  then reads stdout lines until the next `{"type":"result"}` event.
  Returns `Response{Text: result.result, Usage: ...}`.
- Usage: per-turn tokens come from the result event's `usage` field
  (`input_tokens`, `output_tokens`, `cache_read_input_tokens`,
  `cache_creation_input_tokens`). `total_cost_usd` is cumulative
  across the session — Session remembers the previous total and
  reports the delta as `CostUSD`, so all existing accumulation
  (stats, debuglog, match-over screen) stays correct unchanged.
- Timeout: caller passes the same 45s ctx used today. On ctx
  expiry, process error, stdout EOF, or malformed framing, `Ask`
  returns an error and the Session is dead (`Alive() == false`);
  the process is killed (WaitDelay-bounded, like the stateless path).
- `Close()`: closes stdin (graceful CLI exit on EOF), waits with a
  bound, kills if needed. Safe on nil/dead sessions. Abrupt parent
  death closes the pipe → child exits on its own; no leak.
- The one-turn-at-a-time TUI flow means no concurrent `Ask` calls;
  Session may assume serial use.

## Adapter gate — `internal/agent/adapter.go`

Sessions are claude-only. `Adapter` gains a nilable capability field
(e.g. `StartSession func(dir string) (*Session, error)` or an
equivalent flag + constructor); codex/gemini/custom keep it nil and
are completely untouched — their stateless path does not change.

## Prompt protocol

- **Turn 1 (priming):** today's full rendered template via
  `RenderPrompt(data)` — identical content, same persona slot, same
  FORMAT RULES (needling default, table-truth rule). No template
  changes.
- **Later decision turns:** compact delta, built from existing
  pieces:
  - when a hand ended since the last turn: its digest summary line
    (e.g. `Hand 3: the human won 220 (you folded).`)
  - `=== CURRENT HAND ===` + `BuildHandState(...)` output
  - `=== TABLE TALK THIS HAND ===` + talk log
  - `=== YOUR MOVE ===` legal actions, min raise, max amount, and a
    one-line reminder: reply with ONLY the JSON object, same format
    and rules as before.
- **Retries:** ADR-0001's retry rides the conversation — the
  reminder goes out as the next turn, so the model sees its own bad
  reply. Same two-attempt budget as today.
- **Reaction turn (match end):** one more session turn asking for
  the one-line reaction (plain text, no JSON) — replaces the second
  cold spawn. Dead session → today's stateless reaction spawn.
- The Go-side `Digest` keeps being maintained exactly as today; it
  is the catch-up payload for session restarts (and still feeds the
  stateless fallback prompts).

Where the full-vs-delta choice lives: the session-aware decision path
knows whether this session has been primed; `GetDecision`'s
render-then-ask flow gains a seam for that (exact shape is a plan
detail). The stateless path keeps calling `RenderPrompt` unchanged.

## Failure & restart policy

- Any session `Ask` failure → that decision completes via the
  existing retry/fallback ladder **stateless** (cold spawn), so a
  match can never stall (ADR-0001). Session marked dead.
- The next agent decision lazily starts a fresh session; its priming
  message is the full template with the current digest — the new
  process catches up on match history. At most one session start
  attempt per decision; if it fails, that decision runs stateless
  and the next decision may try again.
- First session start is also lazy (on the first agent decision).

## TUI / main wiring

- `Model` holds the session (nil until first use, claude only).
- Call sites keep the decorator pattern:
  `ask := debuglog.WrapAsker(<session-or-stateless asker>, m.log)` —
  debuglog and all logging fields work unchanged (cache_read/write
  will now show real values in transcripts).
- Match over (or program exit): session closed. tea returns the
  final model to main; main calls the model's session-close hook.
- `--quiet` unaffected (quiet suppresses say/reaction, not the
  session mechanics).

## Testing

- Stub CLI in testdata: a small script/binary speaking the
  stream-json protocol (reads JSONL user messages, emits scripted
  `assistant` + `result` events). Unit tests drive Session against
  it: turn framing (one result per Ask), usage parsing + cumulative
  cost delta math, timeout → dead + process killed, Close on EOF,
  restart-after-death flow, no-session adapters unaffected.
- Existing GetDecision/GetReaction tests keep passing untouched
  (stateless path unchanged).
- Live verification (manual, per debug-transcript protocol): one
  `--debug` sonnet match; expect `cache_read > 0` on decision 2+,
  per-decision cost ~$0.001-0.003 in `agent_call` events, and the
  reaction turn on the session.

## Measured baselines (2026-07-04, for regression comparison)

- Stateless sonnet: ~$0.015/decision, 1.8-4.9s, cache_read always 0.
- Two-turn stream-json spike: turn 1 write 2069 tokens ($0.0125),
  turn 2 read 2069 / write 76 (~$0.0012), model retained turn-1
  state; result events carry per-turn usage; needs `--verbose`.
