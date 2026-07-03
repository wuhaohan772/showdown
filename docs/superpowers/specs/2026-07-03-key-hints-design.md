# Key Hint Rows — Design

Date: 2026-07-03
Status: approved

## Purpose

`t` (trash talk) now works during the agent's turn and at hand end, but
nothing on screen says so — the options row only renders on the player's
betting turn. Discoverability gap: the player has no way to learn the
key exists outside their turn. Goal: every phase where keys are live
shows a bottom `>` key row, same pattern as the existing your-turn row.

## Changes (all in `internal/tui/app.go`)

`View`'s bottom phase switch gains two cases:

- `phaseAgentTurn`: render `  > (t)alk`
- `phaseHandEnd`: render `  > (enter) next hand  (t)alk`

`finishHand` banners drop the now-redundant `. enter for next hand`
suffix (both the split-pot and the winner variants) — the key row
carries that instruction; the banner stays pure event text.

Unchanged: `phaseHumanTurn` row, input rows, `phaseMatchOver` (already
shows `enter/q to exit`), `phaseRunout` (no live keys — no row), quiet
mode (hints still shown; `t` works in quiet).

## Testing

- View in `phaseAgentTurn` contains `> (t)alk`.
- View in `phaseHandEnd` contains `> (enter) next hand  (t)alk`, and the
  banner no longer contains `enter for next hand`.
- Existing tests unchanged.
