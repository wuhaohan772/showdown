# Motion Pass (Machine Room + Felt) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the showdown TUI a "moving" feel: cards that deal/flow into place, chip-count tweens, typewriter trash talk, a blinking process cursor, and a banner that stamps bright then settles — all driven by one 50ms animation ticker.

**Architecture:** A single `animTickMsg` ticker (started on demand, self-stopping when nothing animates) advances all animation state each frame. Animation state lives in small pure types in a new `internal/tui/anim.go` (`cardSlide` for deal/board slide-ins, `stepToward` for integer tweens) so logic is unit-testable without bubbletea. `app.go` wires the states into the model; `render.go` gains a slide-aware card-row renderer and a chip-pile renderer. A `SHOWDOWN_REDUCE_MOTION=1` env var disables all motion (instant everything).

**Tech Stack:** Go, bubbletea (value-receiver `Update` pattern), lipgloss, bubbles spinner. No new dependencies.

## Global Constraints

- Follow the existing bubbletea pattern exactly: `func (m Model) Update(...)` value receiver, pointer-receiver helpers called on the local copy (e.g. `cmd := m.startHand()`).
- All new animation logic must be inert when `motion == false` (env `SHOWDOWN_REDUCE_MOTION=1`): identical behavior to today, zero extra ticks.
- `--quiet` continues to mean "no table talk": typewriter never runs under quiet, but gameplay motion (deal, tweens) still does.
- One animation ticker only. Never schedule `animTick()` from two places at once; all scheduling goes through `startAnim()` / the `animTickMsg` handler.
- Existing 500ms showdown runout reveal (`runoutTick`) stays as-is.
- Colors: keep existing 16-color codes for existing elements; chips use 256-color `220` (lipgloss degrades automatically on dumb terminals).
- `gofmt` clean; tests colocated in package `tui`; run `go test ./internal/tui/` after every task, full `go test ./...` before the final commit.
- Commit after every task with the message given in the task.

## Deal choreography (reference for Tasks 1–4)

- Frame interval: 50ms (`animTick`).
- A sliding card starts 12 columns right of its slot and moves left 4 columns per frame: offsets 12 → 8 → 4 → 0, then it "lands" (4 visible frames ≈ 200ms per card).
- Hole deal order: agent card 1, agent card 2, human card 1, human card 2 (one `cardSlide` with `total=4`; agent row shows `min(landed,2)` cards, human row `landed-2` clamped to [0,2]).
- Sliding cards render face-down; a card renders by the row's normal reveal rules the moment it lands (human hole cards flip up on landing; agent's stay hidden; board cards flip up on landing).
- While the hole deal runs the model sits in a new `phaseDealing`; action keys are ignored (global `q`/`ctrl+c` still work); when the slide finishes the model calls `advance()` as `startHand` does today.
- Board cards (flop 3, turn 1, river 1) slide in with the same mechanism via a second `cardSlide`, started from the `apply()` funnel when `len(hand.Board)` grows outside the runout path. This does NOT gate input — cards flow in while play continues.

---

### Task 1: Pure animation primitives (`anim.go`)

**Files:**
- Create: `internal/tui/anim.go`
- Test: `internal/tui/anim_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `stepToward(cur, target int) int`; `type cardSlide struct{ total, landed, offset int }`; `newCardSlide(from, total int) cardSlide`; methods `(*cardSlide).tick()`, `(cardSlide) active() bool`. Zero-value `cardSlide{}` is inactive. Constants `slideStart = 12`, `slideStep = 4`, `animFrame = 50 * time.Millisecond`.

- [ ] **Step 1: Write the failing tests**

```go
package tui

import "testing"

func TestStepToward(t *testing.T) {
	cases := []struct{ cur, target, want int }{
		{0, 100, 25},   // quarter of the distance
		{99, 100, 100}, // small gaps still move (min step 1)
		{100, 0, 75},   // works downward
		{100, 99, 99},  // min step -1
		{5, 5, 5},      // at target: no move
	}
	for _, c := range cases {
		if got := stepToward(c.cur, c.target); got != c.want {
			t.Errorf("stepToward(%d,%d) = %d, want %d", c.cur, c.target, got, c.want)
		}
	}
}

func TestCardSlideSequence(t *testing.T) {
	s := newCardSlide(0, 2)
	if !s.active() {
		t.Fatal("fresh slide must be active")
	}
	// Card 1: offsets 12 (initial), then 8, 4, 0 after ticks; next tick lands it.
	wantOffsets := []int{8, 4, 0}
	for i, w := range wantOffsets {
		s.tick()
		if s.offset != w || s.landed != 0 {
			t.Fatalf("tick %d: offset=%d landed=%d, want offset=%d landed=0", i+1, s.offset, s.landed, w)
		}
	}
	s.tick() // lands card 1, card 2 starts at slideStart
	if s.landed != 1 || s.offset != slideStart {
		t.Fatalf("after landing: landed=%d offset=%d, want 1/%d", s.landed, s.offset, slideStart)
	}
	for s.active() {
		s.tick()
	}
	if s.landed != 2 {
		t.Errorf("finished slide landed=%d, want 2", s.landed)
	}
	s.tick() // ticking an inactive slide is a no-op
	if s.landed != 2 {
		t.Errorf("tick after done changed state: %+v", s)
	}
}

func TestCardSlideStartsPartial(t *testing.T) {
	s := newCardSlide(3, 5) // turn/river: board already has 3 landed
	if s.landed != 3 || !s.active() {
		t.Fatalf("partial slide: landed=%d active=%v, want 3/true", s.landed, s.active())
	}
}

func TestZeroValueCardSlideInactive(t *testing.T) {
	var s cardSlide
	if s.active() {
		t.Error("zero-value cardSlide must be inactive")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestStepToward|TestCardSlide|TestZeroValue' -v`
Expected: FAIL — `undefined: stepToward`, `undefined: newCardSlide`.

- [ ] **Step 3: Write the implementation**

```go
// Package-internal animation primitives. Pure state machines, no bubbletea:
// the single animTick loop in app.go advances them once per frame.
package tui

import "time"

const (
	animFrame  = 50 * time.Millisecond
	slideStart = 12 // columns right of the slot a dealt card starts at
	slideStep  = 4  // columns moved left per frame
)

// stepToward moves cur a quarter of the way to target (minimum 1), giving
// count-up/count-down tweens an ease-out feel. cur == target returns cur.
func stepToward(cur, target int) int {
	d := target - cur
	if d == 0 {
		return cur
	}
	step := d / 4
	if step == 0 {
		if d > 0 {
			step = 1
		} else {
			step = -1
		}
	}
	return cur + step
}

// cardSlide animates cards landing one at a time into a row. landed counts
// fully-arrived cards; offset is the extra indent of the currently sliding
// card. The zero value is inactive.
type cardSlide struct {
	total, landed, offset int
}

func newCardSlide(from, total int) cardSlide {
	return cardSlide{total: total, landed: from, offset: slideStart}
}

func (s cardSlide) active() bool { return s.landed < s.total }

func (s *cardSlide) tick() {
	if !s.active() {
		return
	}
	s.offset -= slideStep
	if s.offset < 0 {
		s.landed++
		s.offset = slideStart
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestStepToward|TestCardSlide|TestZeroValue' -v`
Expected: PASS (all four).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/anim.go internal/tui/anim_test.go
git commit -m "feat(tui): animation primitives — integer tween and card-slide state"
```

---

### Task 2: Slide-aware card row + chip pile (`render.go`)

**Files:**
- Modify: `internal/tui/render.go`
- Test: `internal/tui/render_test.go`

**Interfaces:**
- Consumes: `RenderCard(c poker.Card, faceUp bool) string` (existing, unchanged), `cardSlide` fields from Task 1.
- Produces: `RenderCardRowDeal(cards []poker.Card, revealed, landed, offset int) string` — renders the first `landed` cards (face up where `i < revealed`), then, if `landed < len(cards)` and `offset >= 0`, one face-down card after `offset` spaces; later cards are omitted entirely. `offset < 0` means "row not currently receiving a card" (only landed cards render). Also `renderChips(pot, bb int) string` — one gold `●` per big blind in the pot, capped at 12, empty string for pot 0 or bb 0.

- [ ] **Step 1: Write the failing tests** (append to `internal/tui/render_test.go`)

```go
func TestRenderCardRowDealSlidingCard(t *testing.T) {
	cards := []poker.Card{{Rank: 14, Suit: poker.Spades}, {Rank: 13, Suit: poker.Hearts}, {Rank: 7, Suit: poker.Diamonds}}
	got := RenderCardRowDeal(cards, 1, 1, 4)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), got)
	}
	// Landed card is face up (A♠ visible), sliding card is a face-down back
	// after 4 spaces of gap, third card absent.
	if !strings.Contains(got, "A♠") {
		t.Errorf("landed card must render face up: %q", got)
	}
	if !strings.Contains(got, "??") {
		t.Errorf("sliding card must render face down: %q", got)
	}
	if strings.Contains(got, "7♦") || strings.Contains(got, "K♥") {
		t.Errorf("undealt/sliding cards must not show faces: %q", got)
	}
	// gap: landed card box (4 wide) + separating space + 4 offset spaces before the back
	if !strings.Contains(lines[0], "┐     ┌") {
		t.Errorf("want 5-space gap (1 join + 4 offset) before sliding card, got %q", lines[0])
	}
}

func TestRenderCardRowDealNoIncoming(t *testing.T) {
	cards := []poker.Card{{Rank: 14, Suit: poker.Spades}, {Rank: 13, Suit: poker.Hearts}}
	got := RenderCardRowDeal(cards, 2, 1, -1) // offset -1: no card currently sliding into this row
	if strings.Contains(got, "??") {
		t.Errorf("offset -1 must not draw an incoming card: %q", got)
	}
	if !strings.Contains(got, "A♠") || strings.Contains(got, "K♥") {
		t.Errorf("exactly the landed cards must render: %q", got)
	}
}

func TestRenderCardRowDealComplete(t *testing.T) {
	cards := []poker.Card{{Rank: 14, Suit: poker.Spades}, {Rank: 13, Suit: poker.Hearts}}
	if got, want := RenderCardRowDeal(cards, 2, 2, -1), RenderCardRow(cards, 2); got != want {
		t.Errorf("fully landed deal row must equal RenderCardRow:\n%q\n%q", got, want)
	}
}

func TestRenderChips(t *testing.T) {
	if got := renderChips(0, 20); got != "" {
		t.Errorf("empty pot: want empty string, got %q", got)
	}
	if got := renderChips(60, 20); strings.Count(got, "●") != 3 {
		t.Errorf("60/20 pot: want 3 chips, got %q", got)
	}
	if got := renderChips(10000, 20); strings.Count(got, "●") != 12 {
		t.Errorf("huge pot: want cap of 12 chips, got %q", got)
	}
	if got := renderChips(100, 0); got != "" {
		t.Errorf("bb 0 must not divide by zero, want empty, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestRenderCardRowDeal|TestRenderChips' -v`
Expected: FAIL — `undefined: RenderCardRowDeal`, `undefined: renderChips`.

- [ ] **Step 3: Write the implementation** (append to `internal/tui/render.go`)

```go
var chipStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))

// RenderCardRowDeal draws a row mid-deal: the first landed cards (face up
// where i < revealed), then — if a card is currently sliding into this row
// (landed < len(cards) && offset >= 0) — one face-down card after offset
// spaces. Cards beyond the sliding one are not drawn at all.
func RenderCardRowDeal(cards []poker.Card, revealed, landed, offset int) string {
	if landed > len(cards) {
		landed = len(cards)
	}
	blocks := make([]string, 0, landed+1)
	for i := 0; i < landed; i++ {
		blocks = append(blocks, RenderCard(cards[i], i < revealed))
	}
	if landed < len(cards) && offset >= 0 {
		pad := strings.Repeat(" ", offset)
		b := strings.Split(RenderCard(cards[landed], false), "\n")
		for i := range b {
			b[i] = pad + b[i]
		}
		blocks = append(blocks, strings.Join(b, "\n"))
	}
	if len(blocks) == 0 {
		return "\n\n"
	}
	rows := make([]string, 3)
	for _, b := range blocks {
		for i, line := range strings.Split(b, "\n") {
			if rows[i] != "" {
				rows[i] += " "
			}
			rows[i] += line
		}
	}
	return strings.Join(rows, "\n")
}

// renderChips draws the pot as a pile of chips, one per big blind, capped so
// the line never crowds the pot number.
func renderChips(pot, bb int) string {
	if bb <= 0 || pot <= 0 {
		return ""
	}
	n := pot / bb
	if n < 1 {
		n = 1
	}
	if n > 12 {
		n = 12
	}
	return chipStyle.Render(strings.Repeat("●", n))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestRenderCardRowDeal|TestRenderChips' -v`
Expected: PASS. Also run the whole package: `go test ./internal/tui/` — existing `RenderCardRow` tests must still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/render.go internal/tui/render_test.go
git commit -m "feat(tui): slide-aware card row renderer and chip pile"
```

---

### Task 3: Anim ticker, hole-card deal phase, reduce-motion switch (`app.go`)

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `cardSlide`, `stepToward`, `animFrame` (Task 1); `RenderCardRowDeal` (Task 2).
- Produces (used by Tasks 4–7): model fields `motion bool`, `animRunning bool`, `deal cardSlide`, `boardDeal cardSlide`, `boardSeen int`, `potShown int`, `stackShown [2]int` (player-indexed: 0 human, 1 agent), `typeIdx int`, `typeShown int`, `bannerAge int`; msg `animTickMsg{}`; funcs `animTick() tea.Cmd`, `(m *Model) startAnim() tea.Cmd`, `(m Model) animating() bool`, `(m Model) potTarget() int`, `(m Model) displayStacks() (agentStack, humanStack int)`, `(m *Model) setBanner(s string)`; new phase constant `phaseDealing` (string name `"dealing"`). Constant `bannerBright = 6`.

- [ ] **Step 1: Write the failing tests** (append to `internal/tui/app_test.go`; reuse the package's existing `newTestModel(t)` helper)

```go
// tick pumps n animation frames through Update.
func tick(t *testing.T, m Model, n int) Model {
	t.Helper()
	for i := 0; i < n; i++ {
		mm, _ := m.Update(animTickMsg{})
		m = mm.(Model)
	}
	return m
}

func TestDealAnimationGatesPhase(t *testing.T) {
	m := newTestModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	if m.phase != phaseDealing {
		t.Fatalf("phase after startHand = %v, want phaseDealing", m.phase)
	}
	// 4 cards x 4 frames each = 16 frames to land everything; one extra frame
	// runs the phase transition. Pump plenty.
	m = tick(t, m, 25)
	if m.phase == phaseDealing {
		t.Fatalf("deal never completed; phase still dealing after 25 frames")
	}
	if m.phase != phaseHumanTurn && m.phase != phaseAgentTurn {
		t.Fatalf("post-deal phase = %v, want a turn phase", m.phase)
	}
}

func TestReduceMotionSkipsDeal(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := newTestModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	if m.phase == phaseDealing {
		t.Fatal("reduce-motion must skip the dealing phase entirely")
	}
	if m.animRunning {
		t.Fatal("reduce-motion must never start the anim ticker")
	}
}

func TestPotTweenReachesTarget(t *testing.T) {
	m := newTestModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	if m.potShown != 0 {
		t.Fatalf("potShown starts at %d, want 0", m.potShown)
	}
	m = tick(t, m, 60) // deal + tween settle well within 60 frames
	if m.potShown != m.potTarget() {
		t.Errorf("potShown = %d after settling, want target %d", m.potShown, m.potTarget())
	}
	if m.potTarget() != m.hand.Pot {
		t.Errorf("in-hand pot target = %d, want live pot %d", m.potTarget(), m.hand.Pot)
	}
}

func TestAnimTickerStopsWhenIdle(t *testing.T) {
	m := newTestModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	m = tick(t, m, 80)
	if m.animating() {
		t.Fatalf("model still animating after 80 frames: %+v", m)
	}
	if m.animRunning {
		t.Error("animRunning must clear once nothing animates")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestDealAnimation|TestReduceMotion|TestPotTween|TestAnimTicker' -v`
Expected: FAIL — `undefined: animTickMsg`, `undefined: phaseDealing`, missing fields.

- [ ] **Step 3: Implement**

3a. Add `phaseDealing` to the phase enum (after `phaseMatchOver` to keep existing iota values stable) and to `phaseName`:

```go
const (
	phaseHumanTurn phase = iota
	phaseRaiseInput
	phaseTalkInput
	phaseAgentTurn
	phaseRunout
	phaseHandEnd
	phaseMatchOver
	phaseDealing
)
```

In `phaseName`, add: `case phaseDealing: return "dealing"`.

3b. Add fields to `Model` (after `feed`/`width` block):

```go
	// Motion pass state. motion=false (SHOWDOWN_REDUCE_MOTION=1) leaves every
	// animation inert. One ticker drives all of it; animRunning guards
	// against scheduling it twice.
	motion      bool
	animRunning bool
	deal        cardSlide // hole cards at hand start (gates phaseDealing)
	boardDeal   cardSlide // street cards flowing in mid-hand
	boardSeen   int       // board length already animated (or shown) — apply() compares
	potShown    int       // displayed pot, tweens toward potTarget
	stackShown  [2]int    // displayed stacks, player-indexed: 0 human, 1 agent
	typeIdx     int       // feed index being typewriter-revealed; -1 idle
	typeShown   int       // runes of feed[typeIdx].text revealed so far
	bannerAge   int       // frames since banner was set; bright while < bannerBright
```

3c. In `NewModel`, initialize (inside the returned literal add `motion: os.Getenv("SHOWDOWN_REDUCE_MOTION") != "1", typeIdx: -1, bannerAge: bannerBright, stackShown: [2]int{startStack, startStack}` — and add `"os"` to imports if not present):

```go
	return Model{
		opp: opp, stats: st, statsPath: statsPath, quiet: quiet, dir: dir,
		personality: personality, log: log,
		rng:   rand.New(rand.NewSource(seed)),
		match: poker.NewMatch(startStack, startSB, handLimit), digest: agent.NewDigest(startStack),
		input: in, spin: sp,
		motion: os.Getenv("SHOWDOWN_REDUCE_MOTION") != "1",
		typeIdx: -1, bannerAge: bannerBright,
		stackShown: [2]int{startStack, startStack},
	}
```

3d. Add the ticker machinery and target helpers (near `runoutTick`):

```go
const bannerBright = 6 // frames the banner stays bright after being set

type animTickMsg struct{}

func animTick() tea.Cmd {
	return tea.Tick(animFrame, func(time.Time) tea.Msg { return animTickMsg{} })
}

// startAnim schedules the animation ticker if anything needs animating and
// it isn't already running. Callers batch its result into whatever cmd they
// return; nil is safe in tea.Batch.
func (m *Model) startAnim() tea.Cmd {
	if !m.motion || m.animRunning || !m.animating() {
		return nil
	}
	m.animRunning = true
	return animTick()
}

func (m Model) animating() bool {
	if !m.motion || m.hand == nil {
		return false
	}
	at, ht := m.displayStacks()
	return m.deal.active() || m.boardDeal.active() || m.typeIdx >= 0 ||
		m.potShown != m.potTarget() ||
		m.stackShown[1] != at || m.stackShown[0] != ht ||
		m.bannerAge < bannerBright
}

// potTarget is what the displayed pot tweens toward: the live pot during a
// hand, 0 once the hand is settled (the chips visually drain to the winner's
// stack, which tweens up at the same time).
func (m Model) potTarget() int {
	p := m.phase
	if p == phaseTalkInput {
		p = m.talkReturn
	}
	if p == phaseHandEnd || p == phaseMatchOver {
		return 0
	}
	return m.hand.Pot
}

// setBanner stamps the banner bright; it decays to the normal style after
// bannerBright frames (see bannerStyleFor, Task 6).
func (m *Model) setBanner(s string) {
	m.banner = s
	m.bannerAge = 0
}
```

3e. Extract `displayStacks` from `renderTable` (move the existing stack/seat logic — `renderTable` will call this from now on; this also de-duplicates the match-over seat-flip handling):

```go
// displayStacks returns the true (target) agent and human stacks for the
// current phase, including the match-over seat-flip correction.
func (m Model) displayStacks() (agentStack, humanStack int) {
	hs, as := m.humanSeat(), m.agentSeat()
	agentStack, humanStack = m.hand.Seats[as].Stack, m.hand.Seats[hs].Stack
	if m.phase == phaseMatchOver {
		agentStack, humanStack = m.match.Stacks[1], m.match.Stacks[0]
	}
	return agentStack, humanStack
}
```

In `renderTable`, replace the stack computation at the top with:

```go
	hs, as := m.humanSeat(), m.agentSeat()
	agentStack, humanStack := m.displayStacks()
	if m.phase == phaseMatchOver {
		hs = m.finalHumanSeat
		as = 1 - hs
	}
	if m.motion {
		agentStack, humanStack = m.stackShown[1], m.stackShown[0]
	}
```

(The seat variables `hs`/`as` keep their existing match-over correction for card rendering; only the stack numbers now come through the tween.)

3f. Rewrite `startHand`'s tail to enter the deal (replace the final `m.revealed = 0; return m.advance()`):

```go
	m.revealed = 0
	m.boardSeen = 0
	m.potShown = 0
	if !m.motion {
		return m.advance()
	}
	m.phase = phaseDealing
	m.deal = newCardSlide(0, 4)
	return m.startAnim()
```

Also change the two `m.banner = ...` assignments in `startHand` to `m.setBanner(...)` — same string arguments. (There is one: `m.banner = fmt.Sprintf("hand %d — blinds %d/%d", ...)`.)

3g. Add the `animTickMsg` case to `Update` (before `tea.KeyMsg`):

```go
	case animTickMsg:
		if !m.animRunning {
			return m, nil // stale tick after the loop stopped
		}
		var cmd tea.Cmd
		m.deal.tick()
		m.boardDeal.tick()
		if m.phase == phaseDealing && !m.deal.active() {
			cmd = m.advance()
		}
		m.potShown = stepToward(m.potShown, m.potTarget())
		at, ht := m.displayStacks()
		m.stackShown[1] = stepToward(m.stackShown[1], at)
		m.stackShown[0] = stepToward(m.stackShown[0], ht)
		if m.typeIdx >= 0 {
			m.typeShown += 2
			if m.typeShown >= len([]rune(m.feed[m.typeIdx].text)) {
				m.typeIdx = -1
			}
		}
		if m.bannerAge < bannerBright {
			m.bannerAge++
		}
		if m.animating() {
			return m, tea.Batch(cmd, animTick())
		}
		m.animRunning = false
		return m, cmd
```

3h. Restart the ticker wherever targets can change. In `Update`:
- `case startHandMsg:` → `return m, tea.Batch(cmd, m.startAnim())`
- `case decisionMsg:` final return → `return m, tea.Batch(cmd, m.startAnim())`
- `case reactionMsg:` final return → `return m, m.startAnim()`
- `case runoutTickMsg:` both `return m, nil` (runout-complete branch) and `return m, runoutTick()` become `return m, m.startAnim()` and `return m, tea.Batch(runoutTick(), m.startAnim())` respectively.
- In `handleKey`, change the two action-committing returns in `phaseHandEnd`/`phaseHumanTurn` paths: in `humanAction`, every `return m, cmd` after an `m.apply(...)` becomes `return m, tea.Batch(cmd, m.startAnim())`; in `handleKey` `case phaseHandEnd:` the `enter` branch becomes `cmd := m.settleAndNext(); return m, tea.Batch(cmd, m.startAnim())`. In `confirmInput`, the final `return m, cmd` becomes `return m, tea.Batch(cmd, m.startAnim())`.

3i. Render the deal. In `renderTable`, replace the two hole-card row writes:

Agent row (currently `b.WriteString(indent(RenderCardRow(m.hand.Hole[as][:], rev), 2) + "\n")`):

```go
	if m.phase == phaseDealing {
		landed, off := m.deal.landed, -1
		if landed > 2 {
			landed = 2
		} else if m.deal.landed < 2 {
			off = m.deal.offset
		}
		b.WriteString(indent(RenderCardRowDeal(m.hand.Hole[as][:], 0, landed, off), 2) + "\n")
	} else {
		b.WriteString(indent(RenderCardRow(m.hand.Hole[as][:], rev), 2) + "\n")
	}
```

Human row (currently `b.WriteString(indent(RenderCardRow(m.hand.Hole[hs][:], 2), 2) + "\n")`):

```go
	if m.phase == phaseDealing {
		landed, off := m.deal.landed-2, -1
		if landed < 0 {
			landed = 0
		} else if m.deal.active() {
			off = m.deal.offset
		}
		b.WriteString(indent(RenderCardRowDeal(m.hand.Hole[hs][:], landed, landed, off), 2) + "\n")
	} else {
		b.WriteString(indent(RenderCardRow(m.hand.Hole[hs][:], 2), 2) + "\n")
	}
```

(Human's landed cards render face-up — `revealed = landed` — so each card flips as it arrives.)

3j. Pot line with tween + chips (replace `fmt.Fprintf(&b, "  pot: %d\n\n", m.hand.Pot)`):

```go
	pot := m.hand.Pot
	if m.motion {
		pot = m.potShown
	}
	_, bb := m.match.Blinds()
	if chips := renderChips(pot, bb); chips != "" {
		fmt.Fprintf(&b, "  pot %s %d\n\n", chips, pot)
	} else {
		fmt.Fprintf(&b, "  pot %d\n\n", pot)
	}
```

3k. Action bar: add `case phaseDealing:` to the `switch m.phase` in `renderTable` — write nothing (empty bar while cards fly).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/ -v 2>&1 | tail -20`
Expected: new tests PASS. Existing tests that assert on `pot:` text or immediate post-startHand phase may FAIL — fix each by either setting `t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")` (for tests about game flow, not motion) or updating the expected string (`pot: 30` → `pot 30`). Do not weaken motion tests to make flow tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): hole-card deal animation, pot/stack tweens, single anim ticker"
```

---

### Task 4: Board cards flow in mid-hand

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `boardDeal`, `boardSeen`, `newCardSlide`, `RenderCardRowDeal` (Tasks 1–3).
- Produces: board slide-in behavior; `apply()` starts `boardDeal` whenever the board grows outside a runout.

- [ ] **Step 1: Write the failing test**

```go
func TestBoardGrowthStartsSlide(t *testing.T) {
	m := newTestModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	m = tick(t, m, 25) // let the hole deal finish
	// Drive the engine to the flop directly through the apply funnel:
	// call + check ends preflop regardless of which seat is which.
	if err := m.apply(poker.Action{Type: poker.Call}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if err := m.apply(poker.Action{Type: poker.Check}); err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(m.hand.Board) != 3 {
		t.Fatalf("expected flop on board, got %d cards", len(m.hand.Board))
	}
	if !m.boardDeal.active() || m.boardDeal.total != 3 || m.boardDeal.landed != 0 {
		t.Errorf("flop must start a 3-card slide from 0, got %+v", m.boardDeal)
	}
	if m.boardSeen != 3 {
		t.Errorf("boardSeen = %d, want 3", m.boardSeen)
	}
}

func TestBoardGrowthNoSlideWhenReduced(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := newTestModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	_ = m.apply(poker.Action{Type: poker.Call})
	_ = m.apply(poker.Action{Type: poker.Check})
	if m.boardDeal.active() {
		t.Error("reduce-motion must not start a board slide")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestBoardGrowth' -v`
Expected: FAIL — `boardDeal` stays inactive (`apply` doesn't start it yet).

- [ ] **Step 3: Implement**

In `apply()` (the action funnel), after the `m.hand.Apply(a)` call and before logging, add:

```go
	if m.motion && err == nil && len(m.hand.Board) > m.boardSeen && m.hand.Result() == nil {
		m.boardDeal = newCardSlide(m.boardSeen, len(m.hand.Board))
	}
	if m.hand.Result() == nil {
		m.boardSeen = len(m.hand.Board)
	}
```

(`m.hand.Result() != nil` means the hand just ended — an all-in runout fills the board at that moment, and the existing 500ms `phaseRunout` reveal owns that case. `finishHand` already sets `m.revealed`; also add `m.boardSeen = len(m.hand.Board)` at the top of `finishHand` so a post-runout render never re-animates.)

In `renderTable`, replace the board row write (currently inside `if len(m.hand.Board) > 0`):

```go
	if len(m.hand.Board) > 0 {
		switch {
		case inRunout:
			b.WriteString(indent(RenderCardRow(m.hand.Board, boardRev), 2) + "\n")
		case m.motion && m.boardDeal.active():
			b.WriteString(indent(RenderCardRowDeal(m.hand.Board, m.boardDeal.landed, m.boardDeal.landed, m.boardDeal.offset), 2) + "\n")
		default:
			b.WriteString(indent(RenderCardRow(m.hand.Board, boardRev), 2) + "\n")
		}
	}
```

(Board cards land face-up: `revealed = landed`.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/ -run 'TestBoardGrowth' -v` → PASS, then `go test ./internal/tui/` → all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): street cards slide onto the board mid-hand"
```

---

### Task 5: Typewriter trash talk + blinking process cursor

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `typeIdx`/`typeShown` fields and their tick logic (Task 3), `feedLineGroups`.
- Produces: agent chat lines reveal at 2 runes/frame with a `▌` cursor; thinking/typing indicator becomes a blinking `▌`.

- [ ] **Step 1: Write the failing tests**

```go
func TestTypewriterRevealsAgentSay(t *testing.T) {
	m := newTestModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	m = tick(t, m, 25)
	say := "you fold AK preflop? bold strategy"
	mm, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check}, say: say})
	m = mm.(Model)
	if m.typeIdx != len(m.feed)-1 || m.typeShown != 0 {
		t.Fatalf("agent say must start a typewriter at the new feed line: idx=%d shown=%d", m.typeIdx, m.typeShown)
	}
	joined := strings.Join(m.feedLines(60), "\n")
	if strings.Contains(joined, say) {
		t.Fatalf("full say visible before any ticks: %q", joined)
	}
	if !strings.Contains(joined, "▌") {
		t.Errorf("typing line must show the cursor: %q", joined)
	}
	m = tick(t, m, len([]rune(say))) // 2 runes/frame: plenty
	if m.typeIdx != -1 {
		t.Fatalf("typewriter never finished: idx=%d shown=%d", m.typeIdx, m.typeShown)
	}
	joined = strings.Join(m.feedLines(60), "\n")
	if !strings.Contains(joined, say) || strings.Contains(joined, "▌") {
		t.Errorf("finished line must be full text without cursor: %q", joined)
	}
}

func TestHumanSayIsInstant(t *testing.T) {
	m := newTestModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	m.feed = append(m.feed, chatLine{who: "you", text: "read it and weep"})
	if m.typeIdx != -1 {
		t.Error("human lines must not start a typewriter")
	}
	if joined := strings.Join(m.feedLines(60), "\n"); !strings.Contains(joined, "read it and weep") {
		t.Errorf("human line must be fully visible immediately: %q", joined)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/tui/ -run 'TestTypewriter|TestHumanSay' -v`
Expected: `TestTypewriterRevealsAgentSay` FAILS (typeIdx never set; full say instantly visible). `TestHumanSayIsInstant` may already pass — fine.

- [ ] **Step 3: Implement**

3a. In `Update`'s `case decisionMsg:`, extend the say-append block:

```go
		if msg.say != "" && !m.quiet {
			m.digest.AddTalk(m.opp.DisplayName, msg.say)
			m.feed = append(m.feed, chatLine{who: m.opp.DisplayName, text: msg.say})
			if m.motion {
				m.typeIdx, m.typeShown = len(m.feed)-1, 0
			}
		}
```

Same change in `case reactionMsg:` (its `m.feed = append(...)` block).

3b. In `feedLineGroups`, reveal partially (replace the non-divider branch):

```go
		style := sayStyle
		if l.who == "you" {
			style = humanSayStyle
		}
		text := l.text
		if i == m.typeIdx {
			r := []rune(text)
			n := m.typeShown
			if n > len(r) {
				n = len(r)
			}
			text = string(r[:n]) + "▌"
		}
		var group []string
		for _, ln := range wrapChat(l.who+": "+text, w) {
			group = append(group, style.Render(ln))
		}
		out = append(out, group)
```

(Change the loop header to `for i, l := range m.feed {` to get the index.)

3c. Blinking cursor for "is thinking…" / "is typing…": in `NewModel`, replace the spinner constructor:

```go
	sp := spinner.New(spinner.WithSpinner(spinner.Spinner{
		Frames: []string{"▌ ", "  "},
		FPS:    500 * time.Millisecond,
	}))
```

(The two render sites — `m.spin.View()+m.opp.DisplayName+" is thinking..."` and the `is typing…` line in `feedLineGroups` — need no change; they pick up the new frames.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/ -run 'TestTypewriter|TestHumanSay' -v` → PASS; `go test ./internal/tui/` → all PASS (fix any test asserting the old spinner glyph).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): typewriter trash talk and blinking process cursor"
```

---

### Task 6: Banner stamp-and-settle + agent model tag

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `setBanner`, `bannerAge`, `bannerBright` (Task 3).
- Produces: `bannerStyleFor(age int) lipgloss.Style`; header line reads `♠ <name> · <model>` when the adapter has a model.

- [ ] **Step 1: Write the failing tests**

```go
func TestSetBannerStampsBright(t *testing.T) {
	m := newTestModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	if m.bannerAge != 0 {
		t.Fatalf("startHand must stamp the banner (age 0), got %d", m.bannerAge)
	}
	m = tick(t, m, bannerBright+1)
	if m.bannerAge < bannerBright {
		t.Errorf("banner age must reach settle threshold, got %d", m.bannerAge)
	}
	if bannerStyleFor(0).Render("x") == bannerStyleFor(bannerBright).Render("x") {
		t.Error("fresh and settled banner styles must differ")
	}
}

func TestHeaderShowsModelTag(t *testing.T) {
	m := newTestModel(t)
	m.opp.Model = "sonnet"
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	table, _ := m.renderTable()
	if !strings.Contains(table, m.opp.DisplayName+" · sonnet") {
		t.Errorf("header must carry the model tag, got:\n%s", table)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/tui/ -run 'TestSetBanner|TestHeaderShows' -v`
Expected: FAIL — `undefined: bannerStyleFor`; header lacks tag.

- [ ] **Step 3: Implement**

3a. Style function (next to the style vars; `bannerStyle` var can be deleted once unreferenced):

```go
var (
	bannerFresh   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	bannerSettled = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
)

// bannerStyleFor stamps a fresh banner bold, settling to the normal green
// after bannerBright animation frames (~300ms).
func bannerStyleFor(age int) lipgloss.Style {
	if age < bannerBright {
		return bannerFresh
	}
	return bannerSettled
}
```

In `renderTable`, replace `b.WriteString(bannerStyle.Render("  "+m.banner) + "\n")` with:

```go
	b.WriteString(bannerStyleFor(m.bannerAge).Render("  "+m.banner) + "\n")
```

3b. Convert every remaining direct `m.banner = ...` assignment to `m.setBanner(...)`: in `finishHand` (both split/win branches), `decisionMsg` fallback banner, and both error banners in `confirmInput`. (Task 3 already converted `startHand`.) Grep to verify none remain: `grep -n "m.banner = " internal/tui/app.go` must return nothing.

3c. Header tag in `renderTable` (replace `fmt.Fprintf(&b, "  ♠ %s   stack: %d\n", m.opp.DisplayName, agentStack)`):

```go
	name := m.opp.DisplayName
	if m.opp.Model != "" {
		name += " · " + m.opp.Model
	}
	fmt.Fprintf(&b, "  ♠ %s   stack: %d\n", name, agentStack)
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/` → all PASS (fix any existing header-string assertions to expect the tag only when the fake adapter sets a model).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): banner stamps bright then settles; header carries model tag"
```

---

### Task 7: Docs, reduce-motion surface, final verification

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: everything shipped in Tasks 1–6.
- Produces: user-facing documentation of the motion pass and the reduce-motion switch.

- [ ] **Step 1: README — add env var to the Options section**

Append a row to the flag table in `README.md`:

```markdown
| `SHOWDOWN_REDUCE_MOTION=1` | Environment variable: skip all animations (deal, tweens, typewriter) |
```

And after the table, add one sentence:

```markdown
Cards are dealt into place, chips tween, and the agent's trash talk types
out live; set `SHOWDOWN_REDUCE_MOTION=1` if you want everything instant.
```

- [ ] **Step 2: Full verification**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all packages build, vet clean, all tests PASS.

- [ ] **Step 3: Manual smoke (requires a human or `SHOWDOWN_AGENT_CMD` fake)**

Run: `SHOWDOWN_AGENT_CMD="echo {\"action\":\"check\",\"say\":\"typed out one rune pair at a time\"}" go run . --hands 2`
Watch for: 4 hole cards sliding in one at a time (yours flipping up on landing), pot chips + number tweening up from 0 as blinds post, flop cards flowing in after preflop closes, the agent's say typing out with a `▌` cursor, banner stamping bold then settling, stacks/pot draining to the winner at hand end. Then `SHOWDOWN_REDUCE_MOTION=1 go run . --hands 1` — everything instant, no dealing pause.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: document the motion pass and SHOWDOWN_REDUCE_MOTION"
```
