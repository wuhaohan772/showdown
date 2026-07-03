# Trash-Talk Echo & Phase Widening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render the player's last trash-talk line on screen and let `t` open talk input during the agent's turn and at hand end, not just on the player's betting turn.

**Architecture:** All changes in `internal/tui/app.go`. Two new `Model` fields: `humanSay` (echo text, lifecycle mirrors `agentSay`) and `talkReturn` (phase to restore when talk input closes). A new `openTalk` helper centralizes entering talk input. The `decisionMsg` handler preserves an open talk input across the agent's action landing, including the showdown/runout sub-case where reveal ticks must keep advancing behind the input box.

**Tech Stack:** Go, Bubble Tea (existing deps only).

Spec: `docs/superpowers/specs/2026-07-03-talk-echo-and-phases-design.md`

## Global Constraints

- No engine (`internal/poker`), digest, or prompt changes; no new agent CLI calls; no new module dependencies.
- Echo line format exactly: `you: "<line>"`, rendered with `dimStyle`, hidden when `--quiet`.
- Empty talk input stays a no-op (no echo, no digest entry); esc cancels and restores the origin phase.
- `t` valid in `phaseHumanTurn`, `phaseHandEnd`, `phaseAgentTurn` only. NOT in `phaseRunout` or `phaseMatchOver`.
- Existing tests must keep passing unchanged.

---

### Task 1: Echo + `talkReturn` + hand-end talk

**Files:**
- Modify: `internal/tui/app.go` (Model struct ~44-70, `startHand` ~96-111, `handleKey` ~274-307, `humanAction` case "t" ~349-354, `confirmInput` ~358-367, `View` YOU section ~497-498)
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: existing `Model` (`phase`, `input textinput.Model`, `digest *agent.Digest`, `quiet`), existing test helpers `testModel(t)`, `key(s)`, `stripANSI(s)`.
- Produces (Task 2 relies on these): `Model.humanSay string`, `Model.talkReturn phase`, method `func (m *Model) openTalk()`; `confirmInput` restores `m.talkReturn` on the talk path; esc restores `m.talkReturn` on the talk path.

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/app_test.go`:

```go
func typeString(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = m2.(Model)
	}
	return m
}

func TestTalkEchoRendersAndClearsNextHand(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("phase = %v, want phaseTalkInput", m.phase)
	}
	m = typeString(t, m, "read em and weep")
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.phase != phaseHumanTurn {
		t.Fatalf("phase after enter = %v, want phaseHumanTurn", m.phase)
	}
	if !strings.Contains(stripANSI(m.View()), `you: "read em and weep"`) {
		t.Error("echo line missing from view")
	}
	if !strings.Contains(m.digest.HandTalk(), "HUMAN: read em and weep") {
		t.Error("talk missing from digest")
	}
	// fold ends the hand; enter starts the next: echo must clear
	m2, _ = m.Update(key("f"))
	m = m2.(Model)
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if strings.Contains(stripANSI(m.View()), "you:") {
		t.Error("echo should clear at next hand start")
	}
}

func TestTalkEchoHiddenInQuietMode(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", nil)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	m = typeString(t, m, "silence")
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if strings.Contains(stripANSI(m.View()), "you:") {
		t.Error("echo must be hidden in quiet mode")
	}
}

func TestTalkAtHandEndReturnsToHandEnd(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f")) // fold -> phaseHandEnd
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase = %v, want phaseHandEnd", m.phase)
	}
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("t at hand end: phase = %v, want phaseTalkInput", m.phase)
	}
	m = typeString(t, m, "lucky fold")
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase after enter = %v, want phaseHandEnd", m.phase)
	}
	if !strings.Contains(m.digest.HandTalk(), "HUMAN: lucky fold") {
		t.Error("hand-end talk missing from digest")
	}
}

func TestTalkEscRestoresOriginPhase(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f")) // -> phaseHandEnd
	m = m2.(Model)
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("esc from hand-end talk: phase = %v, want phaseHandEnd", m.phase)
	}
}
```

Note: `key("enter")` works because the existing helper builds a `KeyRunes` message whose `String()` is `"enter"`, which `handleKey`'s `case "enter"` matches before falling through to the input-update default. For esc use `tea.KeyMsg{Type: tea.KeyEsc}` as written — it is the real key event and unambiguous.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestTalkEcho|TestTalkAtHandEnd|TestTalkEsc' -v`
Expected: FAIL — echo line missing; `t` at hand end leaves `phaseHandEnd` (talk input never opens); enter after turn-talk returns `phaseHumanTurn` in all cases so `TestTalkAtHandEndReturnsToHandEnd` fails on the re-entry assertion.

- [ ] **Step 3: Implement**

In `internal/tui/app.go`:

3a. Model struct — add two fields:

```go
	phase      phase
	talkReturn phase // phase to restore when talk input closes
	input      textinput.Model
	spin       spinner.Model
	agentSay   string
	humanSay   string
	banner     string
	revealed   int
	saved      bool
```

3b. `startHand` — clear echo next to `m.agentSay = ""`:

```go
	m.agentSay = ""
	m.humanSay = ""
```

3c. New helper (place directly above `confirmInput`):

```go
// openTalk switches to talk input, remembering which phase to restore
// when the input closes (enter or esc).
func (m *Model) openTalk() {
	m.talkReturn = m.phase
	m.phase = phaseTalkInput
	m.input.Placeholder = "talk trash"
	m.input.SetValue("")
	m.input.Focus()
}
```

3d. `humanAction` case "t" — replace the four-line body with:

```go
	case "t":
		m.openTalk()
```

3e. `handleKey` — esc restores `talkReturn` for talk (raise keeps old behavior), and `phaseHandEnd` gains `t`:

```go
		case "esc":
			if m.phase == phaseTalkInput {
				m.phase = m.talkReturn
			} else {
				m.phase = phaseHumanTurn
			}
			m.input.Blur()
			return m, nil
```

```go
	case phaseHandEnd:
		if k == "t" {
			m.openTalk()
			return m, nil
		}
		if k == "enter" {
			cmd := m.settleAndNext()
			return m, cmd
		}
```

3f. `confirmInput` talk path — set echo, restore `talkReturn`:

```go
	if m.phase == phaseTalkInput {
		if val != "" {
			m.digest.AddTalk("HUMAN", val)
			m.humanSay = val
		}
		m.phase = m.talkReturn
		return m, nil
	}
```

3g. `View` — split the YOU line's trailing blank line and insert the echo:

Replace:

```go
	b.WriteString(indent(RenderCardRow(m.hand.Hole[hs][:], 2), 2) + "\n")
	fmt.Fprintf(&b, "  ♥ YOU   stack: %d\n\n", humanStack)
```

with:

```go
	b.WriteString(indent(RenderCardRow(m.hand.Hole[hs][:], 2), 2) + "\n")
	fmt.Fprintf(&b, "  ♥ YOU   stack: %d\n", humanStack)
	if m.humanSay != "" && !m.quiet {
		b.WriteString(dimStyle.Render(`  you: "`+m.humanSay+`"`) + "\n")
	}
	b.WriteString("\n")
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestTalkEcho|TestTalkAtHandEnd|TestTalkEsc' -v`
Expected: PASS (4 tests). Then full suite: `go build ./... && go test ./...` — all green (existing tests unchanged).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: echo player trash talk on screen; allow t at hand end"
```

---

### Task 2: Talk during agent turn + decision-race + runout sub-case

**Files:**
- Modify: `internal/tui/app.go` (`handleKey` ~280-306, `Update` `decisionMsg` case ~234-251, `Update` `runoutTickMsg` case ~257-267)
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `Model.talkReturn`, `Model.humanSay`, `(m *Model) openTalk()` from Task 1; existing `decisionMsg{act, say, fallback}`, `runoutTickMsg`, `phaseRunout`, `m.revealed`.
- Produces: no new API — behavior only.

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/app_test.go`:

```go
func TestTalkDuringAgentTurnSurvivesDecision(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("c")) // human limps -> agent (BB) turn
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase = %v, want phaseAgentTurn", m.phase)
	}
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("t during agent turn: phase = %v, want phaseTalkInput", m.phase)
	}
	m = typeString(t, m, "hurry up")
	// agent's decision lands mid-typing: BB checks, flop comes, agent acts first
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check}})
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("decision clobbered talk input: phase = %v", m.phase)
	}
	if m.hand.Street != poker.Flop {
		t.Fatalf("street = %v, want Flop (decision must still apply)", m.hand.Street)
	}
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase after enter = %v, want phaseAgentTurn (post-decision phase)", m.phase)
	}
	if !strings.Contains(m.digest.HandTalk(), "HUMAN: hurry up") {
		t.Error("talk missing from digest")
	}
}

func TestTalkSurvivesShowdownRunout(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("a")) // human shoves -> agent turn
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase = %v, want phaseAgentTurn", m.phase)
	}
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	m = typeString(t, m, "gg")
	// agent calls the shove mid-typing -> showdown -> runout
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Call}})
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("phase = %v, want phaseTalkInput preserved over runout", m.phase)
	}
	if m.talkReturn != phaseRunout {
		t.Fatalf("talkReturn = %v, want phaseRunout", m.talkReturn)
	}
	// ticks must advance the reveal behind the input box
	for i := 0; i < len(m.hand.Board); i++ {
		m2, _ = m.Update(runoutTickMsg{})
		m = m2.(Model)
	}
	if m.revealed != len(m.hand.Board) {
		t.Fatalf("revealed = %d, want %d (ticks must advance while typing)", m.revealed, len(m.hand.Board))
	}
	if m.phase != phaseTalkInput || m.talkReturn != phaseHandEnd {
		t.Fatalf("after full reveal: phase = %v talkReturn = %v, want phaseTalkInput/phaseHandEnd", m.phase, m.talkReturn)
	}
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase after enter = %v, want phaseHandEnd", m.phase)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestTalkDuringAgentTurn|TestTalkSurvivesShowdown' -v`
Expected: FAIL — `t` during `phaseAgentTurn` is ignored (no case), so `phase` stays `phaseAgentTurn`.

- [ ] **Step 3: Implement**

In `internal/tui/app.go`:

3a. `handleKey` — add a `phaseAgentTurn` case (after the `phaseHumanTurn` case):

```go
	case phaseAgentTurn:
		if k == "t" {
			m.openTalk()
		}
```

3b. `Update` `decisionMsg` case — preserve open talk input across the decision:

```go
	case decisionMsg:
		m.log.Log("agent_decision", map[string]any{
			"action": string(msg.act.Type), "to": msg.act.To,
			"say": msg.say, "fallback": msg.fallback,
		})
		wasTyping := m.phase == phaseTalkInput
		if err := m.apply(msg.act); err != nil {
			// GetDecision guarantees legality; a failure here is a bug — force fallback.
			_ = m.apply(agent.FallbackAction(m.hand.LegalActions()))
		}
		if msg.say != "" && !m.quiet {
			m.agentSay = msg.say
			m.digest.AddTalk(m.opp.DisplayName, msg.say)
		}
		if msg.fallback {
			m.banner = "agent glitched — forced " + string(msg.act.Type)
		}
		cmd := m.advance()
		if wasTyping {
			// The player was mid-sentence: keep the input open and remember
			// the phase the game advanced to for when it closes.
			m.talkReturn = m.phase
			m.phase = phaseTalkInput
		}
		return m, cmd
```

3c. `Update` `runoutTickMsg` case — reveal keeps advancing while typing over a runout:

```go
	case runoutTickMsg:
		typingOverRunout := m.phase == phaseTalkInput && m.talkReturn == phaseRunout
		if m.phase == phaseRunout || typingOverRunout {
			m.revealed++
			if m.revealed >= len(m.hand.Board) {
				m.revealed = len(m.hand.Board)
				if typingOverRunout {
					m.talkReturn = phaseHandEnd
				} else {
					m.phase = phaseHandEnd
				}
				return m, nil
			}
			return m, runoutTick()
		}
		return m, nil
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestTalkDuringAgentTurn|TestTalkSurvivesShowdown' -v`
Expected: PASS. Then full suite: `go build ./... && go test ./...` — all green.

- [ ] **Step 5: Manual smoke test**

Run: `SHOWDOWN_AGENT_CMD="./scripts/stub-agent.sh" go run . --debug`
During the agent's "thinking" spinner press `t`, type a line, enter; at hand end press `t` again. Expected: your line appears as `you: "…"` under YOU; no freeze during a showdown runout; debug log's next `agent_call` prompt contains `HUMAN: <your line>` in the TABLE TALK section.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: allow trash talk during agent turn; survive decision and runout races"
```
