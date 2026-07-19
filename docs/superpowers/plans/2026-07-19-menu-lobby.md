# Menu Lobby (Game-Table Mirror) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the pre-game menu as a casino-table lobby that mirrors the in-game table (opponent seat top, stakes center, your seat bottom) with an entry deal animation and ambient motion.

**Architecture:** All changes live in `internal/tui/menu.go` + `menu_test.go`. The existing `menuModel` state machine (focus, cycling, prefill, `result()`) is untouched; the View is rewritten around three seat zones with the old rendering preserved verbatim as a narrow/unsized fallback. A menu-local ticker (`menuTickMsg` at `animFrame`) drives one entry `cardSlide` and stat tweens, then keeps running for ambient effects (dealer-button blink, chip shimmer) — both pure functions of a frame counter. Spec: `docs/superpowers/specs/2026-07-19-menu-lobby-design.md`.

**Tech Stack:** Go, bubbletea, lipgloss. Reuses motion-pass primitives already in package `tui`: `cardSlide`, `newCardSlide`, `stepToward`, `animFrame`, `RenderCardRowDeal`, `chipStyle`. No new dependencies, no changes to `app.go`/`render.go`/`anim.go`.

## Global Constraints

- `SHOWDOWN_REDUCE_MOTION=1` → `motion=false`: entry instant (cards landed, stats final), ambient never runs, `Init()` returns nil, a stray `menuTickMsg` is inert. Zero ticks.
- Single ticker chain: only `Init()` starts it; the `menuTickMsg` handler returns exactly one next tick (motion on) or nil (motion off). No other code schedules it.
- All existing `menuModel` behavior — focus order, `cycle`, prefill, `result()`, `ErrMenuQuit`, `RunMenu` signature — byte-identical. This is a View + anim-state change only.
- Narrow fallback: `width < 50` (including the never-sized width 0) renders the old flat list byte-identical to today's View, so existing view tests pass untouched.
- Entry animation never gates input; keys work from frame 0.
- Existing test helpers: `menuRoster()`, `noPersonaFile(t)`, `stripANSI(s)` in `menu_test.go` — use them; do not redefine.
- gofmt clean; `go test ./internal/tui/` green after every task; full `go build ./... && go vet ./... && go test ./...` in the final task.
- Commit after every task with the message given.

## Layout reference (≥ 50 cols; from the spec)

```
  ♠ SHOWDOWN                      lifetime vs claude: 3–2 · $1.84

    ┌──┐ ┌──┐    ♠ claude · sonnet ●
    │??│ │??│      persona: needler
    └──┘ └──┘

       ── stakes ─────────────
       stack    ◀ 1500 ▶
       blinds   10/20
       hands    unlimited
       pot ●●●

    ┌──┐ ┌──┐    ♥ YOU
    │A♠│ │A♥│      table talk: on
    └──┘ └──┘

  [enter] deal me in      [q] leave table
```

Focused row's value renders `◀ value ▶` bold; the model segment of the
opponent name line is hidden while "(default)" and unfocused. The `●`
after the opponent name is the blinking dealer button (ambient). The pot
pile is decorative: `blindIdx + 2` chips, one shimmering periodically.

---

### Task 1: Menu anim state + ticker

**Files:**
- Modify: `internal/tui/menu.go`
- Test: `internal/tui/menu_test.go`

**Interfaces:**
- Consumes: `cardSlide`, `newCardSlide`, `stepToward`, `animFrame` (package `tui`, from the motion pass); `stats.Record` fields `Wins`, `Losses`, `CostUSD`.
- Produces (Task 2/3 rely on these exact names): `menuModel` fields `motion bool`, `frames int`, `width int`, `deal cardSlide`, `statW, statL, statC int` (cost in cents); msg `menuTickMsg{}`; `func menuTick() tea.Cmd`; `Init()` returning the first tick when motion is on.

- [ ] **Step 1: Write the failing tests** (append to `internal/tui/menu_test.go`)

```go
func TestMenuTickAdvancesDealAndStats(t *testing.T) {
	key := menuRoster()[0].Key
	st := stats.Stats{key: stats.Record{Wins: 8, Losses: 4, CostUSD: 2.40}}
	m := newMenuModel(menuRoster(), st, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	if !m.motion {
		t.Fatal("motion must default on")
	}
	if !m.deal.active() {
		t.Fatal("motion menu must open with an active entry deal")
	}
	if m.Init() == nil {
		t.Fatal("Init must schedule the menu ticker when motion is on")
	}
	for i := 0; i < 40; i++ {
		mm, cmd := m.Update(menuTickMsg{})
		m = mm.(menuModel)
		if cmd == nil {
			t.Fatalf("frame %d: menu ticker must keep rescheduling (ambient)", i)
		}
	}
	if m.deal.active() {
		t.Error("entry deal must complete within 40 frames")
	}
	if m.statW != 8 || m.statL != 4 || m.statC != 240 {
		t.Errorf("stats must tween to the real record, got W=%d L=%d C=%d", m.statW, m.statL, m.statC)
	}
	if m.frames != 40 {
		t.Errorf("frames = %d, want 40", m.frames)
	}
}

func TestMenuReduceMotionInert(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	if m.motion {
		t.Fatal("SHOWDOWN_REDUCE_MOTION=1 must disable menu motion")
	}
	if m.Init() != nil {
		t.Fatal("Init must not schedule a ticker under reduce-motion")
	}
	mm, cmd := m.Update(menuTickMsg{})
	m = mm.(menuModel)
	if cmd != nil || m.frames != 0 {
		t.Error("a stray tick must be inert under reduce-motion")
	}
}

func TestMenuStoresWindowSize(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, DefaultHandLimit, false, noPersonaFile(t))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(menuModel)
	if m.width != 80 {
		t.Errorf("width = %d, want 80", m.width)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestMenuTick|TestMenuReduceMotion|TestMenuStoresWindow' -v`
Expected: FAIL — `undefined: menuTickMsg`, unknown fields `motion`/`deal`/etc.

- [ ] **Step 3: Implement**

3a. Add fields to `menuModel` (after `talkOn`):

```go
	// Lobby motion state (spec 2026-07-19-menu-lobby-design). motion=false
	// (SHOWDOWN_REDUCE_MOTION=1) renders the final state with zero ticks.
	motion bool
	frames int       // monotonic frame counter driving ambient phases
	width  int       // last tea.WindowSizeMsg; 0 = never sized
	deal   cardSlide // entry deal: opponent's two backs, then your A♠ A♥
	statW  int       // tweened lifetime wins for the focused opponent
	statL  int       // tweened lifetime losses
	statC  int       // tweened lifetime cost in cents
```

3b. In `newMenuModel`, before `return m`:

```go
	m.motion = os.Getenv("SHOWDOWN_REDUCE_MOTION") != "1"
	if m.motion {
		m.deal = newCardSlide(0, 4)
	}
```

3c. Replace `func (m menuModel) Init() tea.Cmd { return nil }` with:

```go
func (m menuModel) Init() tea.Cmd {
	if m.motion {
		return menuTick()
	}
	return nil
}

type menuTickMsg struct{}

func menuTick() tea.Cmd {
	return tea.Tick(animFrame, func(time.Time) tea.Msg { return menuTickMsg{} })
}
```

Add `"time"` to menu.go's imports.

3d. In `Update`, the current code type-asserts `tea.KeyMsg` and returns early for everything else. Replace the top of `Update` with:

```go
func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case menuTickMsg:
		if !m.motion {
			return m, nil
		}
		m.frames++
		m.deal.tick()
		r := m.st[m.roster[m.oppIdx].Key]
		m.statW = stepToward(m.statW, r.Wins)
		m.statL = stepToward(m.statL, r.Losses)
		m.statC = stepToward(m.statC, int(r.CostUSD*100+0.5))
		return m, menuTick()
	case tea.KeyMsg:
		return m.handleMenuKey(msg)
	}
	return m, nil
}

func (m menuModel) handleMenuKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	// ... existing key switch body moves here unchanged ...
	}
	return m, nil
}
```

(Move the existing `switch k.String()` body — up/down/left/right/enter/q — into `handleMenuKey` verbatim.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestMenuTick|TestMenuReduceMotion|TestMenuStoresWindow' -v` → PASS.
Then: `go test ./internal/tui/` — all existing menu tests must still pass (View untouched so far).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/menu.go internal/tui/menu_test.go
git commit -m "feat(tui): menu anim state — entry deal slide, stat tweens, menu ticker"
```

---

### Task 2: Lobby view with flat fallback

**Files:**
- Modify: `internal/tui/menu.go`
- Test: `internal/tui/menu_test.go`

**Interfaces:**
- Consumes: Task 1 fields (`motion`, `frames`, `width`, `deal`, `statW/L/C`); `RenderCardRowDeal(cards []poker.Card, revealed, landed, offset int) string`; `handsLabel`, `menuDim` (existing).
- Produces: `View()` dispatching on `width < lobbyMinWidth` (const `50`) to `viewFlat()` (old rendering, byte-identical) or the lobby; helpers `viewHeader`, `viewSeatOpponent`, `viewStakes`, `viewSeatYou`, `focusVal(row int, v string) string`, `dealLanded() (landed, offset int)`, `statVals() (w, l, cents int)`, `joinSeat`, `padTo`, `onOff`. Task 3 modifies `viewSeatOpponent` (dealer button) and adds shimmer inside `menuChips` — `menuChips() string` is created here returning a plain pile.

- [ ] **Step 1: Write the failing tests**

```go
func TestMenuLobbyView(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1") // final state, no anim noise
	key := menuRoster()[0].Key
	name := menuRoster()[0].DisplayName
	st := stats.Stats{key: stats.Record{Wins: 3, Losses: 2, CostUSD: 1.84}}
	m := newMenuModel(menuRoster(), st, "", "", 1500, 10, 0, false, noPersonaFile(t))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(menuModel)
	v := stripANSI(m.View())
	for _, want := range []string{
		"♠ SHOWDOWN", "lifetime vs " + key + ": 3–2 · $1.84",
		"persona:", "stack", "blinds", "10/20", "hands", "unlimited",
		"table talk: on", "♥ YOU", "A♠", "A♥", "??", "pot ●",
		"[enter] deal me in", "[q] leave table",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("lobby view missing %q in:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "◀ "+name+" ▶") {
		t.Errorf("focused opponent row must show ◀ %s ▶ in:\n%s", name, v)
	}
	// model hidden while "(default)" and unfocused
	if strings.Contains(v, menuDefaultModel) {
		t.Errorf("default model must be hidden when unfocused:\n%s", v)
	}
	// focusing the model row reveals it
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mm.(menuModel)
	if v := stripANSI(m.View()); !strings.Contains(v, "◀ "+menuDefaultModel+" ▶") {
		t.Errorf("focused model row must show ◀ (default) ▶ in:\n%s", v)
	}
}

func TestMenuNarrowFallsBack(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", 1500, 10, 0, false, noPersonaFile(t))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	m = mm.(menuModel)
	v := stripANSI(m.View())
	if strings.Contains(v, "┌──┐") {
		t.Errorf("narrow view must not draw cards:\n%s", v)
	}
	for _, want := range []string{"opponent", "model", "persona", "stack", "blind", "hands", "table talk"} {
		if !strings.Contains(v, want) {
			t.Errorf("narrow fallback missing label %q:\n%s", want, v)
		}
	}
}

func TestMenuLobbyRendersMidDeal(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", 1500, 10, 0, false, noPersonaFile(t))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(menuModel)
	for i := 0; i < 3; i++ { // mid-deal: card 1 still sliding
		mm, _ = m.Update(menuTickMsg{})
		m = mm.(menuModel)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "♠ SHOWDOWN") {
		t.Errorf("mid-deal lobby render broken:\n%s", v)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestMenuLobby|TestMenuNarrow' -v`
Expected: FAIL — lobby strings absent (View still renders the flat list).

- [ ] **Step 3: Implement**

3a. Rename the existing `View()` body: `func (m menuModel) viewFlat() string { ... }` — content byte-identical to today's `View`. New dispatcher plus lobby:

```go
const lobbyMinWidth = 50

func (m menuModel) View() string {
	if m.width < lobbyMinWidth {
		return m.viewFlat()
	}
	var b strings.Builder
	b.WriteString(m.viewHeader() + "\n\n")
	b.WriteString(m.viewSeatOpponent() + "\n\n")
	b.WriteString(m.viewStakes() + "\n")
	b.WriteString(m.viewSeatYou() + "\n\n")
	b.WriteString(menuDim.Render("  [enter] deal me in      [q] leave table") + "\n")
	return b.String()
}
```

3b. Focus affordance + small helpers:

```go
var menuFocus = lipgloss.NewStyle().Bold(true)

// focusVal marks the focused row's value with ◀ ▶; others render bare.
func (m menuModel) focusVal(row int, v string) string {
	if m.focus == row {
		return menuFocus.Render("◀ " + v + " ▶")
	}
	return v
}

// dealLanded is the entry deal's progress; reduce-motion is always done.
func (m menuModel) dealLanded() (landed, offset int) {
	if !m.motion {
		return 4, -1
	}
	return m.deal.landed, m.deal.offset
}

// statVals is the header's lifetime record: tweened under motion, live
// otherwise (also live per-frame targets are recomputed in the tick).
func (m menuModel) statVals() (w, l, cents int) {
	if !m.motion {
		r := m.st[m.roster[m.oppIdx].Key]
		return r.Wins, r.Losses, int(r.CostUSD*100 + 0.5)
	}
	return m.statW, m.statL, m.statC
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// padTo right-pads s to display width w (ANSI-aware).
func padTo(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}
```

3c. Header:

```go
func (m menuModel) viewHeader() string {
	w, l, cents := m.statVals()
	line := fmt.Sprintf("lifetime vs %s: %d–%d · $%.2f",
		m.roster[m.oppIdx].Key, w, l, float64(cents)/100)
	left := "  ♠ SHOWDOWN"
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(line) - 2
	if pad < 2 {
		pad = 2
	}
	return left + strings.Repeat(" ", pad) + menuDim.Render(line)
}
```

3d. Seats and stakes:

```go
// joinSeat lays two text lines alongside a 3-line card block.
func joinSeat(cardBlock, line1, line2 string) string {
	cards := strings.Split(cardBlock, "\n")
	for len(cards) < 3 {
		cards = append(cards, "")
	}
	const cardW = 9 // "┌──┐ ┌──┐"
	return "    " + padTo(cards[0], cardW) + "    " + line1 + "\n" +
		"    " + padTo(cards[1], cardW) + "    " + line2 + "\n" +
		"    " + padTo(cards[2], cardW)
}

func (m menuModel) viewSeatOpponent() string {
	landed, off := m.dealLanded()
	oppLanded, oppOff := landed, -1
	if oppLanded > 2 {
		oppLanded = 2
	} else if landed < 2 {
		oppOff = off
	}
	cards := RenderCardRowDeal([]poker.Card{{}, {}}, 0, oppLanded, oppOff)
	name := "♠ " + m.focusVal(rowOpponent, m.roster[m.oppIdx].DisplayName)
	if mv := m.modelOpts[m.oppIdx][m.modelIdx]; m.focus == rowModel || mv != menuDefaultModel {
		name += " · " + m.focusVal(rowModel, mv)
	}
	persona := "  persona: " + m.focusVal(rowPersona, m.personaOpts[m.personaIdx])
	return joinSeat(cards, name, persona)
}

func (m menuModel) viewStakes() string {
	sb := m.blindOpts[m.blindIdx]
	var b strings.Builder
	b.WriteString(menuDim.Render("       ── stakes ─────────────") + "\n")
	fmt.Fprintf(&b, "       stack    %s\n", m.focusVal(rowStack, fmt.Sprintf("%d", m.stackOpts[m.stackIdx])))
	fmt.Fprintf(&b, "       blinds   %s\n", m.focusVal(rowBlind, fmt.Sprintf("%d/%d", sb, sb*2)))
	fmt.Fprintf(&b, "       hands    %s\n", m.focusVal(rowHands, handsLabel(m.handOpts[m.handIdx])))
	fmt.Fprintf(&b, "       pot %s\n", m.menuChips())
	return b.String()
}

// menuChips is the decorative pot pile: grows with the blind preset.
// (Task 3 adds the shimmer.)
func (m menuModel) menuChips() string {
	n := m.blindIdx + 2
	return chipStyle.Render(strings.Repeat("●", n))
}

func (m menuModel) viewSeatYou() string {
	landed, off := m.dealLanded()
	youLanded, youOff := landed-2, -1
	if youLanded < 0 {
		youLanded = 0
	} else if landed < 4 {
		youOff = off
	}
	hole := []poker.Card{{Rank: 14, Suit: poker.Spades}, {Rank: 14, Suit: poker.Hearts}}
	cards := RenderCardRowDeal(hole, youLanded, youLanded, youOff)
	talk := "  table talk: " + m.focusVal(rowTalk, onOff(m.talkOn))
	return joinSeat(cards, "♥ YOU", talk)
}
```

Add `"github.com/haohanwu/showdown/internal/poker"` to menu.go's imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestMenuLobby|TestMenuNarrow' -v` → PASS.
Then the whole package: `go test ./internal/tui/` — the old `TestMenuView` must pass UNCHANGED (its model is never sized, width 0 → `viewFlat`). If it fails, `viewFlat` is not byte-identical to the old `View` — fix `viewFlat`, do not touch the test.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/menu.go internal/tui/menu_test.go
git commit -m "feat(tui): menu lobby view — three-seat table layout with flat fallback"
```

---

### Task 3: Ambient effects + final verification

**Files:**
- Modify: `internal/tui/menu.go`
- Test: `internal/tui/menu_test.go`

**Interfaces:**
- Consumes: Task 2's `viewSeatOpponent` and `menuChips`; `frames` counter (Task 1); `chipStyle` (render.go).
- Produces: dealer-button blink (`dealerButton() string`, ~1s period: on while `frames%20 < 10`), chip shimmer inside `menuChips` (active while `frames%60 < 4`, bright chip index `(frames/60) % n`), style `menuShimmer`.

- [ ] **Step 1: Write the failing tests**

```go
func TestMenuDealerButtonBlinks(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", 1500, 10, 0, false, noPersonaFile(t))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(menuModel)
	m.frames = 5 // blink-on half of the period
	on := strings.Count(stripANSI(m.View()), "●")
	m.frames = 15 // blink-off half
	off := strings.Count(stripANSI(m.View()), "●")
	if on != off+1 {
		t.Errorf("dealer button must add exactly one ● in the on-phase: on=%d off=%d", on, off)
	}
}

func TestMenuChipShimmer(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", 1500, 10, 0, false, noPersonaFile(t))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(menuModel)
	m.frames = 0 // shimmer active (0%60 < 4), dealer blink on (0%20 < 10)
	a := m.View()
	m.frames = 64 // shimmer inactive (64%60 = 4), dealer blink on (64%20 = 4)
	b := m.View()
	if a == b {
		t.Error("shimmer frame must render differently from non-shimmer frame")
	}
	if stripANSI(a) != stripANSI(b) {
		t.Error("shimmer must be color-only: stripped output must be identical")
	}
}

func TestMenuNoAmbientUnderReduceMotion(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", 1500, 10, 0, false, noPersonaFile(t))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(menuModel)
	m.frames = 5
	a := m.View()
	m.frames = 15
	if b := m.View(); a != b {
		t.Error("reduce-motion menu must render identically at any frame")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestMenuDealer|TestMenuChipShimmer|TestMenuNoAmbient' -v`
Expected: `TestMenuDealerButtonBlinks` FAILS (on == off, no button yet); `TestMenuChipShimmer` FAILS (a == b). `TestMenuNoAmbientUnderReduceMotion` may already pass.

- [ ] **Step 3: Implement**

3a. Dealer button; append it to the opponent name line in `viewSeatOpponent` (after the model segment, before `joinSeat`):

```go
var menuShimmer = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Bold(true)

// dealerButton blinks beside the opponent seat on a ~1s period (20 frames
// at animFrame). Static (absent) under reduce-motion.
func (m menuModel) dealerButton() string {
	if !m.motion || m.frames%20 >= 10 {
		return ""
	}
	return " " + chipStyle.Render("●")
}
```

In `viewSeatOpponent`, change the name construction to end with:

```go
	name += m.dealerButton()
```

3b. Shimmer in `menuChips` (replace the Task 2 body):

```go
// menuChips is the decorative pot pile: grows with the blind preset; one
// chip shimmers bright for a few frames every ~3s (color-only, so layout
// never shifts). Static under reduce-motion.
func (m menuModel) menuChips() string {
	n := m.blindIdx + 2
	shimmer := -1
	if m.motion && m.frames%60 < 4 {
		shimmer = (m.frames / 60) % n
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i == shimmer {
			b.WriteString(menuShimmer.Render("●"))
		} else {
			b.WriteString(chipStyle.Render("●"))
		}
	}
	return b.String()
}
```

Note: lipgloss may drop colors entirely in a no-color test environment, which would make `TestMenuChipShimmer`'s `a != b` assertion fail the same way Task 6 of the motion pass did. If `a == b` with the implementation in place, force a color profile for that test only:

```go
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)
```

(import `"github.com/muesli/termenv"` in menu_test.go). Prefer this over weakening the assertion.

- [ ] **Step 4: Run tests + full verification**

Run: `go test ./internal/tui/ -run 'TestMenuDealer|TestMenuChipShimmer|TestMenuNoAmbient' -v` → PASS.
Then: `go build ./... && go vet ./... && go test ./...` → all green, gofmt clean (`gofmt -l internal/tui/`).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/menu.go internal/tui/menu_test.go
git commit -m "feat(tui): menu ambient motion — dealer-button blink and chip shimmer"
```
