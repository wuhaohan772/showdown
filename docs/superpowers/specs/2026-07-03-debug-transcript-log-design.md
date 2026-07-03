# Debug Transcript Log — Design

Date: 2026-07-03
Status: approved

## Purpose

Showdown's Bubble Tea TUI owns the terminal, so agent prompts, raw CLI
replies, engine transitions, and human input are invisible during play.
Debugging a reported bug today relies entirely on the player's memory.
This feature adds an opt-in JSONL transcript so one reproduction of a
bug yields complete, greppable evidence.

Success criterion: for a reproducible in-game bug, the user provides a
one-line "saw X, expected Y", a hand number, and the log file — and
that is sufficient to diagnose engine, agent-pipeline, and most
TUI-state bugs without further back-and-forth.

## Activation

- New CLI flag `--debug` in `main.go`; env var `SHOWDOWN_DEBUG=1` acts
  as an alias (either enables logging).
- When enabled, create `~/.showdown/debug-<YYYYMMDD-HHMMSS>.jsonl`
  (same directory as `stats.json`).
- After the TUI exits, `main.go` prints the log file path to stderr so
  the user can find it (the alt screen hides output during play).
- If the log file cannot be created, print a warning to stderr and run
  without logging. Logging must never prevent play.

## Logger (`internal/debuglog`)

New package with a single type:

```go
type Logger struct { /* mutex + *os.File + seq counter */ }

func New(path string) (*Logger, error)
func (l *Logger) Log(event string, fields map[string]any) // nil-safe
func (l *Logger) Close() error                            // nil-safe
```

- Each `Log` call writes one JSON line:
  `{"seq": N, "t": "<RFC3339Nano>", "event": "<name>", ...fields}`.
- A nil `*Logger` is a no-op for all methods, so call sites carry no
  `if debug` conditionals.
- Write errors are dropped silently (best-effort logging; the game
  never crashes or blocks on the log).
- `WrapAsker(ask agent.Asker, l *Logger) agent.Asker` decorator: logs
  event `agent_call` with the full prompt, raw CLI output, error string
  (if any), and wall-clock duration, then passes results through
  unchanged.

## Events and hook points

All hooks live in `internal/tui/app.go` unless noted.

| Event | Where | Fields |
|---|---|---|
| `session_start` | model creation (`NewModel`) | agent key/name, quiet flag, cwd, RNG seed |
| `hand_start` | hand setup (`startHand`) | hand #, blinds, button seat, both hole cards, stacks |
| `agent_call` | `WrapAsker` around `Adapter.Asker` (decision + reaction call sites) | prompt, raw output, error, duration_ms |
| `agent_decision` | after `GetDecision` returns | parsed action, amount, say, fallback_used |
| `apply` | new `m.apply(a)` helper funneling every `m.hand.Apply` call | actor, action, amount, error (if any), then post-state: hand #, street, pot, stacks, bets, board |
| `human_key` | `Update` on `tea.KeyMsg` | key string, current TUI state |
| `hand_end` | result available | hand #, winner, pot, resulting stacks |

Notes:

- The RNG seed is generated in `main`/`NewModel` as today
  (`time.Now().UnixNano()`), logged once. Seed **storage only** — a
  `--seed` replay flag is out of scope (YAGNI until a bug needs it).
- Cards are captured incrementally: hole cards at `hand_start`, board
  via the `apply` post-state snapshots as streets deal. No deck dump
  needed.
- The seven existing `m.hand.Apply` call sites collapse into the single
  `m.apply` helper; behavior is otherwise identical.

## Plumbing

`main.go` creates the `*Logger` (or nil), passes it to `tui.NewModel`.
`Model` stores it and threads it to the asker wrap points. `main.go`
closes the logger and prints its path after `Run()` returns.

## Bug report protocol (user-facing)

Per bug, the user supplies:

1. One line: "saw X, expected Y".
2. Hand number (or approximate wall-clock time).
3. The log file (`~/.showdown/debug-*.jsonl`).
4. A screenshot only when the bug is visual.

The log is ground truth; the description points into it. Diagnosis
greps by hand number / seq.

## Privacy

The log contains full prompts, including the match digest and
table-talk history. It is a local file under `~/.showdown/`, never
transmitted or committed. (It lives outside the repo, so no
`.gitignore` change is required.)

## Testing

- `internal/debuglog`: emits valid JSONL with monotonically increasing
  `seq`; nil logger is a no-op; write-after-close does not panic;
  `WrapAsker` passes prompt/response/error through unchanged and logs
  on both success and error paths.
- `internal/tui`: existing flow tests still pass with a nil logger; the
  `m.apply` funnel applies actions identically to the previous direct
  calls (covered by existing tests once call sites are swapped).
