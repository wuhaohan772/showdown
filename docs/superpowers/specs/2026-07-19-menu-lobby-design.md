# Menu Lobby Design — Game-Table Mirror

Date: 2026-07-19. Status: approved (direction "casino table lobby", depth
"seated at the table", motion "ambient + entry", layout option A).

## Goal

Replace the menu's plain settings list with a lobby that mirrors the
in-game table: the menu IS the table before the deal. Opponent seat on
top, stakes in the middle where the pot lives, your seat at the bottom.
Pressing enter seats you — menu → game reads as the same camera.

## Layout (≥ 50 columns)

```
  ♠ SHOWDOWN                      lifetime vs claude: 3W-2L · $1.84

    ┌──┐ ┌──┐    ♠ claude · sonnet
    │??│ │??│      persona: needler
    └──┘ └──┘

         ── stakes ──────────────
         stack   ◀ 1500 ▶
         blinds    10/20
         hands     unlimited
         pot ●●●

    ┌──┐ ┌──┐    ♥ YOU
    │A♠│ │A♥│      table talk: on
    └──┘ └──┘

  [enter] deal me in      [q] leave table
```

- **Header**: `♠ SHOWDOWN` left; right-aligned lifetime stats for the
  currently focused opponent (from `stats.Stats.Line`), updating live as
  the opponent cycles.
- **Opponent seat**: two face-down cards via `RenderCard`; name line
  `♠ <opponent> · <model>` (model omitted when "(default)"); persona
  line beneath. Opponent, model, and persona are three focusable rows
  rendered in place within this seat.
- **Stakes block**: `stack`, `blinds` (value shown as `sb/sb*2`),
  `hands` (`unlimited` for 0) — each focusable. Below them a decorative
  `pot` chip pile: `renderChips` fed a small multiple of the selected
  blind so the pile subtly grows with the stakes. Never focusable.
- **Your seat**: A♠ A♥ rendered face-up (fixed decorative hand — the
  house always deals you rockets in the lobby), `♥ YOU` line,
  `table talk: on|off` focusable row.
- **Footer**: `[enter] deal me in      [q] leave table`, dim style.
- **Focus affordance**: the focused row's value renders `◀ value ▶` and
  bold; unfocused rows render the bare value — same affordance as the
  old menu, relocated.

## Narrow fallback (< 50 columns or width unknown)

Cards and chip pile drop; the seven rows render as today's flat list
(label + value + ◀ ▶ on focus). Nothing else degrades. The menu must
receive `tea.WindowSizeMsg` to learn its width; width 0 (never sized)
uses the fallback.

## Interaction — unchanged semantics

- up/k, down/j: walk focus across the 7 settings in spatial order:
  opponent → model → persona → stack → blind → hands → talk.
- left/h, right/l: cycle the focused value (wrapping), exactly the
  existing `cycle` behavior.
- enter: start match. q / ctrl+c: quit via `ErrMenuQuit`.
- All existing `menuModel` state, prefill (`--model`, `--personality`,
  `--stack`, `--blind`, `--hands`, `--quiet`), and `result()` mapping
  are untouched. This is a View + row-placement change.

## Motion

Reuses the motion-pass primitives (`cardSlide`, `stepToward`,
`animFrame`) and the same single-ticker discipline (a menu-local
`menuTickMsg` at `animFrame`, scheduled only while something animates
or an ambient beat is due; guard flag prevents double-scheduling).

- **Entry (once, on open)**: the four cards deal in via one
  `cardSlide(0, 4)` — opponent's two backs, then your A♠ A♥ (face-up on
  landing, same convention as the game's deal). Lifetime stats tween up
  from zero via `stepToward` — wins, losses, and cost as integer cents
  (rendered `$%.2f` from cents/100). Roughly 0.6–0.8s total. Input is live from the first frame — the entry
  animation never gates keys; any settings change renders normally
  mid-deal.
- **Ambient (loops)**: a dealer-button `●` beside the opponent seat
  blinks on a ~1s period; every few seconds one pot chip renders
  bright for a couple frames (shimmer). Both are pure render effects
  driven by a frame counter — no state machine.
- **Reduce motion** (`SHOWDOWN_REDUCE_MOTION=1`): entry is instant
  (cards landed, stats final), ambient never runs, no ticker ever
  starts. Menu behaves exactly like a static screen.

## Architecture

Everything stays in `internal/tui/menu.go` (+ `menu_test.go`):

- `menuModel` gains: `motion bool`, `tickRunning bool`, `frames int`
  (monotonic frame counter driving ambient phases), `deal cardSlide`
  (entry), `statShown` ints for the tweened stats.
- `Init()` returns the first tick cmd when motion is on.
- `Update` gains a `menuTickMsg` case (advance deal, tween stats,
  increment frames, reschedule while animating or ambient active) and a
  `tea.WindowSizeMsg` case (store width).
- `View` rewritten around the three-seat layout; helper functions per
  zone (`viewSeatOpponent`, `viewStakes`, `viewSeatYou`) kept small.
- No new animation primitives; no changes to app.go or render.go.

Ambient means the ticker runs for the menu's lifetime (motion on). This
is accepted: the menu is short-lived and the tick is 20fps of pure
re-render.

## Testing

- Existing menu tests keep passing unmodified where they exercise
  state/prefill/result; View-string assertions updated to the new row
  rendering.
- New tests: entry deal completes and cards land (frames pumped via the
  tick msg); reduce-motion renders final state immediately with no
  ticker; focus walk order matches the spatial order; narrow fallback
  (width < 50) renders all 7 labels; stats tween reaches the true
  values.

## Out of scope

Sound, mouse, per-opponent art, menu → game shared-process transition
(menu remains a separate tea.Program), any change to flags or the
`--agent` fast path.
