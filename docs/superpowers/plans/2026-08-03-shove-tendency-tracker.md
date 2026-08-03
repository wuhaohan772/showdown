# Shove-Tendency Tracker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the agent a persistent, cross-hand read on how often the opponent shoves all-in and how often the agent has folded to it, surfaced in the match digest and weighted by a new prompt rule, so the agent starts exploiting a bluff-heavy shover instead of folding to the same move every time.

**Architecture:** A pure detection function (`agent.DetectShove`) inspects one finished hand's action log + result to classify it as "opponent shoved" / "agent folded to shove". Two new counters on the existing `Digest` type accumulate this across the match and render one extra line in `Digest.String()` once there's enough sample (≥3 hands). One new prompt-template bullet tells the model how to weigh that line. No changes to the numeric poker engine (`internal/poker`) — this only changes what data reaches the LLM prompt.

**Tech Stack:** Go. Existing packages: `internal/poker` (hand/result types), `internal/agent` (digest, prompt template), `internal/tui` (wires hand-finish into the digest).

## Global Constraints

- Spec: `docs/superpowers/specs/2026-08-03-shove-tendency-tracker-design.md`.
- No changes to `internal/poker` — detection reads existing `LogItem`/`Result` fields only.
- Digest line omitted below 3 hands played, and omitted entirely if `opponentShoves == 0` — keep the digest token-lean (project's existing token-compression-first cost posture).
- New prompt-template bullet must land as the last *behavioral* Format-rules bullet, immediately before the final "Your entire reply must parse as JSON." formatting bullet, and must NOT disturb the existing invariant that the needling-doctrine bullet is the last thing before it (tested by `TestNeedlingRuleLastInFormatRules` in `internal/agent/prompt_test.go`) — recency-wins is a proven-fragile mechanism on haiku for that rule specifically.
- Follow existing test style in this package: table-driven `t.Run`-free loops with `t.Errorf`, package `agent`, import `github.com/wuhaohan772/showdown/internal/poker` where poker types are needed (see `internal/agent/decide_test.go`).

---

### Task 1: `DetectShove` pure function

**Files:**
- Create: `internal/agent/shove.go`
- Test: `internal/agent/shove_test.go`

**Interfaces:**
- Consumes: `poker.LogItem{Seat int, Street poker.Street, Act poker.Action, AllIn bool}`, `poker.Result{Winner int, Split bool, Pot int, Desc string, Showdown bool}` (both already defined in `internal/poker/hand.go`, no changes).
- Produces: `func DetectShove(log []poker.LogItem, agentSeat int, r *poker.Result) (sawShove, foldedToShove bool)` — Task 3 calls this directly.

- [ ] **Step 1: Write the failing tests**

```go
package agent

import (
	"testing"

	"github.com/wuhaohan772/showdown/internal/poker"
)

func TestDetectShove(t *testing.T) {
	cases := []struct {
		name           string
		log            []poker.LogItem
		agentSeat      int
		result         *poker.Result
		wantSaw        bool
		wantFoldedToIt bool
	}{
		{
			name: "opponent shoves, agent folds",
			log: []poker.LogItem{
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Raise, To: 3000}, AllIn: true},
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Fold}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: 1, Pot: 3000},
			wantSaw:        true,
			wantFoldedToIt: true,
		},
		{
			name: "opponent shoves, agent calls and wins at showdown",
			log: []poker.LogItem{
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Raise, To: 3000}, AllIn: true},
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Call}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: 0, Pot: 6000, Showdown: true, Desc: "pair of kings"},
			wantSaw:        true,
			wantFoldedToIt: false,
		},
		{
			name: "no shove in the hand",
			log: []poker.LogItem{
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Call}},
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Check}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: 1, Pot: 200, Showdown: true, Desc: "ace high"},
			wantSaw:        false,
			wantFoldedToIt: false,
		},
		{
			name: "split pot after opponent shove",
			log: []poker.LogItem{
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Raise, To: 3000}, AllIn: true},
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Call}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: -1, Split: true, Pot: 6000, Showdown: true, Desc: "chop"},
			wantSaw:        true,
			wantFoldedToIt: false,
		},
		{
			name: "agent itself shoves — must not count as opponent shove",
			log: []poker.LogItem{
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Raise, To: 3000}, AllIn: true},
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Fold}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: 0, Pot: 3000},
			wantSaw:        false,
			wantFoldedToIt: false,
		},
	}
	for _, c := range cases {
		saw, folded := DetectShove(c.log, c.agentSeat, c.result)
		if saw != c.wantSaw || folded != c.wantFoldedToIt {
			t.Errorf("%s: DetectShove() = (%v, %v), want (%v, %v)", c.name, saw, folded, c.wantSaw, c.wantFoldedToIt)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/agent/ -run TestDetectShove -v`
Expected: FAIL — `undefined: DetectShove` (compile error).

- [ ] **Step 3: Write the minimal implementation**

```go
package agent

import "github.com/wuhaohan772/showdown/internal/poker"

// DetectShove classifies a finished hand's log from agentSeat's perspective:
// sawShove is true if the opponent went all-in at any point in the hand;
// foldedToShove is true if that hand also ended with the agent folding.
func DetectShove(log []poker.LogItem, agentSeat int, r *poker.Result) (sawShove, foldedToShove bool) {
	for _, li := range log {
		if li.Seat != agentSeat && li.AllIn {
			sawShove = true
			break
		}
	}
	if sawShove && !r.Showdown && !r.Split && r.Winner != agentSeat {
		foldedToShove = true
	}
	return sawShove, foldedToShove
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/agent/ -run TestDetectShove -v`
Expected: PASS (all 5 cases).

- [ ] **Step 5: Commit**

```bash
git add internal/agent/shove.go internal/agent/shove_test.go
git commit -m "feat: detect opponent all-in shoves from a finished hand's log"
```

---

### Task 2: `Digest` counters and rendered tendency line

**Files:**
- Modify: `internal/agent/digest.go`
- Test: `internal/agent/digest_test.go` (new file)

**Interfaces:**
- Consumes: nothing new (no dependency on Task 1's `DetectShove` — this task only adds the counters and rendering; Task 3 is what calls `DetectShove` and feeds its output in).
- Produces: `func (d *Digest) RecordHand(sawShove, foldedToShove bool)` — Task 3 calls this once per finished hand, right next to the existing `EndHand` call. `Digest.String()` behavior changes (additive line) but its signature is unchanged.

- [ ] **Step 1: Write the failing tests**

Create `internal/agent/digest_test.go`:

```go
package agent

import (
	"strings"
	"testing"
)

func TestDigestTendencyLineOmittedBelowThreshold(t *testing.T) {
	d := NewDigest(3000)
	d.RecordHand(true, true) // hand 1: shove, agent folded
	d.RecordHand(true, true) // hand 2: shove, agent folded
	if strings.Contains(d.String(), "Opponent tendency") {
		t.Errorf("digest with only 2 hands recorded should omit tendency line, got %q", d.String())
	}
}

func TestDigestTendencyLineOmittedWhenNoShoves(t *testing.T) {
	d := NewDigest(3000)
	for i := 0; i < 5; i++ {
		d.RecordHand(false, false)
	}
	if strings.Contains(d.String(), "Opponent tendency") {
		t.Errorf("digest with zero shoves should omit tendency line, got %q", d.String())
	}
}

func TestDigestTendencyLineAtThreshold(t *testing.T) {
	d := NewDigest(3000)
	d.RecordHand(true, true)  // shove, folded
	d.RecordHand(true, false) // shove, called
	d.RecordHand(false, false)
	want := "Opponent tendency: shoved all-in 2 of 3 hands; you folded to 1 of those shoves."
	if !strings.Contains(d.String(), want) {
		t.Errorf("digest = %q, want it to contain %q", d.String(), want)
	}
}

func TestDigestTendencyLineUpdatesAcrossMoreHands(t *testing.T) {
	d := NewDigest(3000)
	for i := 0; i < 6; i++ {
		d.RecordHand(true, i < 4) // 6 shoves, 4 folds
	}
	want := "Opponent tendency: shoved all-in 6 of 6 hands; you folded to 4 of those shoves."
	if !strings.Contains(d.String(), want) {
		t.Errorf("digest = %q, want it to contain %q", d.String(), want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/agent/ -run TestDigestTendency -v`
Expected: FAIL — `d.RecordHand undefined (type *Digest has no field or method RecordHand)`.

- [ ] **Step 3: Write the minimal implementation**

Modify `internal/agent/digest.go`:

```go
package agent

import (
	"fmt"
	"strings"
)

// Digest is the agent's stateless "memory": hand summaries plus table talk.
type Digest struct {
	events     []string
	handTalk   []string
	startStack int

	handsPlayed    int
	opponentShoves int
	foldedToShove  int
}

func NewDigest(startStack int) *Digest { return &Digest{startStack: startStack} }

func (d *Digest) AddEvent(line string) { d.events = append(d.events, line) }

func (d *Digest) AddTalk(speaker, line string) {
	d.handTalk = append(d.handTalk, speaker+": "+line)
}

func (d *Digest) HandTalk() string {
	if len(d.handTalk) == 0 {
		return "(nothing said yet)"
	}
	return strings.Join(d.handTalk, "\n")
}

// RecordHand accumulates one finished hand's shove read (see DetectShove)
// into the running match-wide tendency counters.
func (d *Digest) RecordHand(sawShove, foldedToShove bool) {
	d.handsPlayed++
	if sawShove {
		d.opponentShoves++
	}
	if foldedToShove {
		d.foldedToShove++
	}
}

// EndHand archives this hand's talk into the running digest and records the summary.
func (d *Digest) EndHand(summary string) {
	for _, t := range d.handTalk {
		d.events = append(d.events, "  talk — "+t)
	}
	d.handTalk = nil
	d.events = append(d.events, summary)
}

func (d *Digest) String() string {
	base := strings.Join(d.events, "\n")
	if len(d.events) == 0 {
		base = fmt.Sprintf("First hand of the match. Stacks even at %d.", d.startStack)
	}
	if d.handsPlayed >= 3 && d.opponentShoves > 0 {
		tendency := fmt.Sprintf("Opponent tendency: shoved all-in %d of %d hands; you folded to %d of those shoves.",
			d.opponentShoves, d.handsPlayed, d.foldedToShove)
		return tendency + "\n" + base
	}
	return base
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/agent/ -v`
Expected: PASS for all `TestDigest*` tests, and no regressions in the pre-existing `TestDigestFirstHandMessageUsesStartStack` (internal/agent/prompt_test.go:71) — that test only calls `NewDigest(3000).String()` with zero hands recorded, which still hits the `len(d.events) == 0` branch and returns the original "First hand of the match..." string, so it must still pass unmodified.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/digest.go internal/agent/digest_test.go
git commit -m "feat: track opponent shove frequency in the match digest"
```

---

### Task 3: Wire detection into hand-finish, add prompt rule

**Files:**
- Modify: `internal/tui/app.go:265-289` (`finishHand`)
- Modify: `internal/agent/prompt_template.md`
- Modify: `internal/agent/prompt_test.go` (extend the existing rule-order test)

**Interfaces:**
- Consumes: `agent.DetectShove(log []poker.LogItem, agentSeat int, r *poker.Result) (sawShove, foldedToShove bool)` (Task 1), `(*agent.Digest).RecordHand(sawShove, foldedToShove bool)` (Task 2).
- Produces: nothing further downstream — this is the last task.

- [ ] **Step 1: Write the failing test (prompt rule order)**

Modify `internal/agent/prompt_test.go`'s `TestNeedlingRuleLastInFormatRules` to also assert the new shove-tendency rule comes after needling but before the final JSON-format instruction:

```go
func TestNeedlingRuleLastInFormatRules(t *testing.T) {
	out := RenderPrompt(RequestData{AgentName: "Stub"})
	needle := strings.Index(out, "Needle them with what you know")
	truth := strings.Index(out, "Trash talk must be true to the table")
	cards := strings.Index(out, "Card talk is a weapon, not a habit")
	shoveRule := strings.Index(out, "Opponent tendency")
	jsonRule := strings.Index(out, "Your entire reply must parse as JSON")
	if needle == -1 {
		t.Fatal("format rules missing the personalized-needling default rule")
	}
	if truth == -1 {
		t.Fatal("format rules missing the table-truth rule (2026-07-04: haiku invented human folds)")
	}
	if cards == -1 {
		t.Fatal("format rules missing the card-talk rule")
	}
	if shoveRule == -1 {
		t.Fatal("format rules missing the opponent-shove-tendency rule")
	}
	if jsonRule == -1 {
		t.Fatal("format rules missing the final JSON-only instruction")
	}
	if !(cards < truth && truth < needle && needle < shoveRule && shoveRule < jsonRule) {
		t.Error("behavioral rule order must be card-talk < table-truth < needling < shove-tendency < final JSON rule")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/agent/ -run TestNeedlingRuleLastInFormatRules -v`
Expected: FAIL — `shoveRule == -1` ("format rules missing the opponent-shove-tendency rule").

- [ ] **Step 3: Add the prompt-template bullet**

Modify `internal/agent/prompt_template.md`. Current last three lines of the Format rules list are:

```
- Needle them with what you know — their projects, their habits, their history from your instructions and memory — but vary the material and don't force it. Land the personal jab when it fits; when it doesn't, talk poker or say nothing. Skip the personal angle entirely only if your persona redirects or silences it.
- Your entire reply must parse as JSON.
```

Insert a new bullet between them so the file reads:

```
- Needle them with what you know — their projects, their habits, their history from your instructions and memory — but vary the material and don't force it. Land the personal jab when it fits; when it doesn't, talk poker or say nothing. Skip the personal angle entirely only if your persona redirects or silences it.
- Opponent tendency, if the MATCH section reports one: a high shove rate paired with folds on your side is a bluff tell, not strength — widen your calling range against the next shove instead of folding again on reputation alone. A low or absent shove rate means a shove is still real strength; keep respecting it.
- Your entire reply must parse as JSON.
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/agent/ -run TestNeedlingRuleLastInFormatRules -v`
Expected: PASS.

- [ ] **Step 5: Write the failing test for the app.go wiring**

The package already has real infrastructure for this (`internal/tui/app_test.go`): `testModel(t) Model` builds a model (line 18), hands are driven via `m.Update(msg)` with `startHandMsg{}` to start the match, `key("a")` for the human to shove all-in (app.go:665-674, legal only when `legal[poker.Bet] || legal[poker.Raise]`), `decisionMsg{act: poker.Action{...}}` to simulate the agent's move (see `TestAgentDecisionMsgApplies`), and `key("enter")` at `phaseHandEnd` to advance to the next hand (see `TestTalkEchoRendersAndPersistsAcrossHands`). `m.hand.LegalActions()` is available to pick a legal non-shove action for whichever seat acts first, since heads-up button position alternates each hand and either seat can be first to act preflop.

Add to `internal/tui/app_test.go`:

```go
// playHumanShoveAgentFold drives one hand to completion where the human
// shoves all-in the first time it's their turn, and the agent folds once
// facing that shove. If the agent acts first (its turn comes before the
// human has shoved), it calls/checks to pass the action along rather than
// folding prematurely.
func playHumanShoveAgentFold(t *testing.T, m Model) Model {
	t.Helper()
	shoved := false
	for m.phase == phaseHumanTurn || m.phase == phaseAgentTurn {
		if m.phase == phaseHumanTurn {
			m2, _ := m.Update(key("a"))
			m = m2.(Model)
			shoved = true
			continue
		}
		// phaseAgentTurn
		act := poker.Action{Type: poker.Fold}
		if !shoved {
			legal := map[poker.ActionType]bool{}
			for _, a := range m.hand.LegalActions() {
				legal[a] = true
			}
			act = poker.Action{Type: poker.Call}
			if !legal[poker.Call] {
				act = poker.Action{Type: poker.Check}
			}
		}
		m2, _ := m.Update(decisionMsg{act: act})
		m = m2.(Model)
	}
	return m
}

func TestFinishHandRecordsShoveTendency(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)

	for hand := 1; hand <= 3; hand++ {
		m = playHumanShoveAgentFold(t, m)
		if m.phase != phaseHandEnd {
			t.Fatalf("hand %d: phase = %v, want phaseHandEnd", hand, m.phase)
		}
		if hand < 3 {
			m2, _ = m.Update(key("enter"))
			m = m2.(Model)
		}
	}

	want := "Opponent tendency: shoved all-in 3 of 3 hands; you folded to 3 of those shoves."
	if d := m.digest.String(); !strings.Contains(d, want) {
		t.Errorf("digest = %q, want it to contain %q", d, want)
	}
}
```

Note: if a run shows the human isn't always the shover (e.g. `playHumanShoveAgentFold` fires when it's actually the agent's turn to shove because of how `decisionMsg` or seat assignment behaves in this harness), adjust the helper based on the actual `-v` output rather than guessing further — this is exactly what the next step's run is for.

- [ ] **Step 6: Run the test to verify it fails**

Run: `go test ./internal/tui/ -run TestFinishHandRecordsShoveTendency -v`
Expected: FAIL (digest never updated — `finishHand` doesn't call `DetectShove`/`RecordHand` yet).

- [ ] **Step 7: Wire the call into `finishHand`**

Modify `internal/tui/app.go`. Current code (lines 265-289):

```go
func (m *Model) finishHand() tea.Cmd {
	r := m.hand.Result()
	m.log.Log("hand_end", map[string]any{
		"hand": m.match.HandNum, "winner_seat": r.Winner,
		"pot": r.Pot, "showdown": r.Showdown, "split": r.Split, "desc": r.Desc,
		"stacks": []int{m.hand.Seats[0].Stack, m.hand.Seats[1].Stack},
	})
	m.phase = phaseHandEnd
	winnerName := "YOU"
	if r.Winner == m.agentSeat() {
		winnerName = m.opp.DisplayName
	}
	how := "opponent folded"
	if r.Showdown {
		how = r.Desc
	}
	if r.Split {
		m.setBanner(fmt.Sprintf("split pot (%d) — %s", r.Pot, r.Desc))
	} else {
		m.setBanner(fmt.Sprintf("%s wins %d (%s)", winnerName, r.Pot, how))
	}
	summary := agentHandSummary(m.match.HandNum, r, r.Winner == m.agentSeat())
	m.digest.EndHand(summary)
	m.pendingResults = append(m.pendingResults, summary)
	m.revealed = len(m.hand.Board)
```

Change the two lines around `agentHandSummary`/`EndHand` to:

```go
	summary := agentHandSummary(m.match.HandNum, r, r.Winner == m.agentSeat())
	sawShove, foldedToShove := agent.DetectShove(m.hand.Log, m.agentSeat(), r)
	m.digest.RecordHand(sawShove, foldedToShove)
	m.digest.EndHand(summary)
	m.pendingResults = append(m.pendingResults, summary)
	m.revealed = len(m.hand.Board)
```

Confirm `internal/tui/app.go` already imports `github.com/wuhaohan772/showdown/internal/agent` (it must, since `m.digest` is `*agent.Digest`) — no new import needed.

- [ ] **Step 8: Run the test to verify it passes**

Run: `go test ./internal/tui/ -run TestFinishHandRecordsShoveTendency -v`
Expected: PASS.

- [ ] **Step 9: Run the full test suite**

Run: `go test ./...`
Expected: PASS, no regressions (specifically `internal/agent` and `internal/tui` packages).

- [ ] **Step 10: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go internal/agent/prompt_template.md internal/agent/prompt_test.go
git commit -m "feat: weigh opponent shove tendency in decisions

Wires DetectShove + Digest.RecordHand into finishHand, and adds a
Format-rules bullet telling the model to read a high shove-rate +
folds as a bluff tell instead of re-reading every shove as strength."
```

- [ ] **Step 11: Manual verification (required — not automatable)**

LLM decision quality cannot be unit-tested. Per the project's existing haiku-canary protocol (see [[model-talk-quality-and-cache]] and README's "Choosing a model" section):

1. Run a live match (`--debug` flag on) against `--model haiku`, scripting/forcing the human side to shove all-in preflop on 3-4 consecutive hands where the agent has a foldable-but-not-trash hand.
2. Inspect the `--debug` JSONL transcript's prompts from hand 4 onward: confirm the "Opponent tendency" line appears in `match_digest` with the expected numbers.
3. Eyeball whether the agent's fold/call choice on the 4th+ shove shows any sign of loosening (calls lighter, or its "say" references the pattern, e.g. "you keep shoving, I don't buy it this time") versus folding identically to hand 1.
4. Repeat once against `--model sonnet` (the documented-more-reliable model) to confirm no regression in its already-correct fold-attribution behavior — specifically re-check the fold-attribution scenario from [[model-talk-quality-and-cache]] (decision right after the agent's own fold) still resolves correctly, since this task touched the same Format-rules block.
5. If the model doesn't visibly react to the new digest line in 3-4 samples, that's a prompt-wording problem, not a wiring bug (the spec's "out of scope" section already anticipates this may need iteration) — note it for a follow-up rather than expanding this plan.

---

## Self-Review Notes

- **Spec coverage:** Section 1 (detection) → Task 1. Section 2 (counters) → Task 2. Section 3 (surfacing/threshold) → Task 2. Section 4 (prompt change) → Task 3. Section 5 (wiring) → Task 3. Section 6 (testing) → each task's test steps + Task 3 Step 11 manual protocol. All covered.
- **Placeholder scan:** no TBD/TODO; Task 3 Step 5's test includes an explicit note to the implementer because `internal/tui`'s existing test harness shape is unknown from the spec alone — this is a real open question (not a placeholder for skippable work), flagged with a concrete fallback (loop `finishHand` 3x) rather than left vague.
- **Type consistency:** `DetectShove(log []poker.LogItem, agentSeat int, r *poker.Result) (sawShove, foldedToShove bool)` used identically in Task 1's implementation and Task 3's call site. `RecordHand(sawShove, foldedToShove bool)` used identically in Task 2 and Task 3.
