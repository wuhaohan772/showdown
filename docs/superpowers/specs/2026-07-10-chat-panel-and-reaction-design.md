# Chat panel + match-end reaction — design

**Date:** 2026-07-10
**Status:** approved-for-planning

## Problem

Table talk is showdown's product gimmick, but it is nearly invisible:

1. **Ephemeral says.** `View` renders one agent say + one human say inline
   (`app.go` ~679, ~698), overwritten on every action. Look away, miss it. The
   3-second linger (`sayLingerDuration`, `saySeq`, `showSayMsg`) is a band-aid
   on this overwrite — it delays the loss, doesn't cure it.
2. **Lost parting shot.** The match-end reaction is the finale, but nothing on
   the match-over screen waits for it. A fast `q` hits `tea.Quit`, killing the
   in-flight reaction cmd before it renders. The reaction's cost re-save is lost
   with it.

## Goals

- Persistent, scrollback-style **chat panel** on the right side of the table so
  banter accumulates and is watchable.
- **Talk-only** feed content: spoken lines (Claude / you) with a dim
  `─ hand N ─` divider between hands. Hand results stay in the table banner.
- Match-over screen **waits** for the reaction with a spinner and blocks a
  premature quit (with an escape hatch).
- Net simplification: the feed makes the linger band-aid obsolete — remove it.

## Non-goals

- Manual scrolling of the feed. Auto-scroll to newest; overflow shows last-K.
- Interleaving game events (folds, blinds) into the feed. Talk-only.
- Reflowing the whole TUI to be fully responsive. One panel width + one narrow
  fallback is enough.

## 1. Data model

UI-side feed store, parallel to and independent of the agent `Digest` (the
digest keeps feeding the agent exactly as today — untouched):

```go
type chatLine struct {
    who     string // "Claude"/DisplayName, "you", or "" for a divider
    text    string // spoken text, or "hand N" for a divider
    divider bool
}
```

`feed []chatLine` lives on `Model`. Append sites:

| Line          | Where appended            |
|---------------|---------------------------|
| agent say     | `decisionMsg` handler     |
| human say     | `confirmInput`            |
| reaction      | `reactionMsg` handler     |
| `─ hand N ─`  | `startHand`               |

`Digest.AddTalk` stays as-is; the feed is a separate UI concern.

## 2. Layout / render

- Add `width, height int` to `Model`, populated from a new
  `tea.WindowSizeMsg` case in `Update` (none exists today).
- **Left column** = the current table string, with the inline say lines
  removed.
- **Panel:** fixed width **30**. Dim `talk` header. Each line wrapped to the
  panel's inner width; speaker prefix `Claude: ` / `you: `; continuation lines
  indented 2. Divider renders as `─ hand N ─` padded to panel width. Newest at
  bottom; if the feed exceeds panel height, render the last-K lines that fit
  (auto-scroll, no manual scroll).
- **Compose:** `lipgloss.JoinHorizontal(lipgloss.Top, left, sep+panel)`. Pad
  the left and panel columns to equal height so the separator/divider bars run
  full-height and the spacing looks clean (the user's precision bar).
- **Narrow / no-size fallback:** when `width < 74` OR `width == 0` (tests,
  pipes, tiny terminals), skip the side panel and render the feed as a
  **bottom log** (last ~4 lines) above the action bar. Same `feed` store, two
  presentations.
- **quiet mode:** no panel and no log — the plain table, exactly as today.

## 3. Match-end reaction

- New `reactionPending bool`, set true when `settleAndNext` dispatches the
  reaction cmd.
- While pending in `phaseMatchOver`, the panel/log shows a spinner line:
  `⠋ Claude is typing…`.
- **Block premature quit:** in `phaseMatchOver`, if `reactionPending`, the
  first `q`/`enter` does **not** quit — it shows the hint
  `(waiting for parting shot — q again to skip)`. A second `q` quits. The
  reaction's existing 45s timeout guarantees the spinner can't hang forever;
  `q`-again is the manual escape hatch.
- `reactionMsg` appends the reaction as the final Claude line and clears
  `reactionPending`. The existing cost re-save in that handler is unchanged; it
  now reliably runs because the screen waited.

## 4. Code removed (net simplification)

The feed supersedes the overwrite band-aid. Delete:

- `sayLingerDuration`, `saySeq`, `showSayMsg` and its `Tick`/`Update` branch
- `agentSay`, `humanSay` fields and their inline render blocks in `View`

## 5. Testing

- **Panel path:** set `m.width` wide, inject a `feed`, assert wrapped speaker
  lines and a `─ hand N ─` divider appear, and that the two columns are equal
  height (no ragged spacing).
- **Fallback path:** `width == 0`, assert the bottom-log renders the last-K
  lines above the action bar.
- **Reaction block:** in `phaseMatchOver` with `reactionPending`, send `q` →
  assert it does **not** quit and the hint shows; send `reactionMsg` → assert
  the feed gains the line and `reactionPending` clears; send `q` → asserts quit.
- **quiet mode:** assert no panel/log renders.

## Decisions

- **Panel width:** fixed **30** (not scaled to terminal). Simpler to make
  pixel-perfect; the narrow fallback covers small terminals. Revisit only if it
  looks cramped on wide terminals in practice.
