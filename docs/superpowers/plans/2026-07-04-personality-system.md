# Personality System (P3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the player choose the opponent's table persona — 5 embedded presets or a user-submitted file — injected into the game prompts with a hard token cap.

**Architecture:** A new `internal/agent/personality.go` owns preset texts and file loading (2,000-char cap). The persona is a plain string threaded: `main.go` resolves it once at startup (flag → picker → default file → `needler`), passes it to `tui.NewModel`, which injects it into every decision prompt (new `{personality}` slot in `prompt_template.md`) and the match-end `ReactionPrompt`. It lives in the USER prompt, never the system prompt — `claudeSystemPrompt` must stay byte-identical for the prompt cache (ADR-0003).

**Tech Stack:** Go, bubbletea TUI. No new dependencies.

## Global Constraints

- All tests green: `go test ./...` after every task.
- Formatting: `gofmt -l .` must print nothing before each commit.
- Commit style: `feat: …` / `docs: …`; final `graph: incremental update for personality feature`.
- `claudeSystemPrompt` in `internal/agent/adapter.go` must NOT change (cache prefix, ADR-0003).
- Preset names, exactly: `needler` (default), `unhinged`, `polite`, `silent`, `degen`. Each preset text ≤ 600 bytes.
- User file cap: `PersonalityCap = 2000` runes; over-cap files are truncated (not rejected) and the caller is told.
- Default resolution when no flag/picker choice: `~/.showdown/personality.md` if it exists, else the `needler` preset.
- Existing behavior preserved: with no flag, no file, and picker defaults, the rendered prompt must carry the same needling/grudge-match energy the current template has.

---

### Task 1: `personality.go` — presets + capped file loading

**Files:**
- Create: `internal/agent/personality.go`
- Test: `internal/agent/personality_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (later tasks rely on these exact names):
  - `const PersonalityCap = 2000`
  - `func PresetNames() []string` — `["needler", "unhinged", "polite", "silent", "degen"]` in that order
  - `func PersonalityPath() string` — `~/.showdown/personality.md`
  - `func LoadPersonality(arg, defaultFile string) (text string, truncated bool, err error)` — arg `""` → defaultFile if it exists else `needler` preset; arg matching a preset name → that preset; anything else → treated as a file path (missing file = error). Files capped at `PersonalityCap` runes.

- [ ] **Step 1: Write the failing test**

```go
// internal/agent/personality_test.go
package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPresetNamesOrderAndContent(t *testing.T) {
	want := []string{"needler", "unhinged", "polite", "silent", "degen"}
	got := PresetNames()
	if len(got) != len(want) {
		t.Fatalf("PresetNames() = %v", got)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("PresetNames()[%d] = %q, want %q", i, got[i], n)
		}
		text, _, err := LoadPersonality(n, "")
		if err != nil || strings.TrimSpace(text) == "" {
			t.Errorf("preset %q: text=%q err=%v", n, text, err)
		}
		if len(text) > 600 {
			t.Errorf("preset %q is %d bytes, cap is 600", n, len(text))
		}
	}
}

func TestLoadPersonalityDefaultsToNeedler(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.md")
	text, truncated, err := LoadPersonality("", missing)
	if err != nil || truncated {
		t.Fatalf("err=%v truncated=%v", err, truncated)
	}
	needler, _, _ := LoadPersonality("needler", "")
	if text != needler {
		t.Errorf("default = %q, want needler preset", text)
	}
}

func TestLoadPersonalityDefaultFileWins(t *testing.T) {
	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("custom persona"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, truncated, err := LoadPersonality("", f)
	if err != nil || truncated || text != "custom persona" {
		t.Errorf("got %q truncated=%v err=%v", text, truncated, err)
	}
}

func TestLoadPersonalityExplicitPathAndCap(t *testing.T) {
	f := filepath.Join(t.TempDir(), "big.md")
	if err := os.WriteFile(f, []byte(strings.Repeat("é", PersonalityCap+500)), 0o644); err != nil {
		t.Fatal(err)
	}
	text, truncated, err := LoadPersonality(f, "")
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Error("over-cap file must report truncated")
	}
	if n := len([]rune(text)); n != PersonalityCap {
		t.Errorf("rune count = %d, want %d", n, PersonalityCap)
	}
}

func TestLoadPersonalityErrors(t *testing.T) {
	if _, _, err := LoadPersonality("no-such-preset-or-file", ""); err == nil {
		t.Error("unknown name must error")
	}
	if _, _, err := LoadPersonality(filepath.Join(t.TempDir(), "missing.md"), ""); err == nil {
		t.Error("missing explicit path must error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run 'Preset|LoadPersonality' -v`
Expected: FAIL — `undefined: PresetNames`, `undefined: LoadPersonality`

- [ ] **Step 3: Write the implementation**

```go
// internal/agent/personality.go
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PersonalityCap bounds user-submitted persona files (in runes). The persona
// rides in every decision prompt, so an uncapped file would silently
// multiply per-call token cost.
const PersonalityCap = 2000

var presetOrder = []string{"needler", "unhinged", "polite", "silent", "degen"}

var presets = map[string]string{
	"needler": "You have an ego. Table talk is live psychology — hold grudges, celebrate, " +
		"seethe, and let it color your play if that's who you are. If they tilt you, that's " +
		"on you. Needle them about their habits, their projects, their weaknesses. This is a " +
		"grudge match.",
	"unhinged": "You are a foul-mouthed tilt monster. Curse freely, swear at bad beats, " +
		"celebrate obnoxiously, and spiral dramatically when the cards go against you. Every " +
		"pot is personal. Profanity yes; slurs never.",
	"polite": "You are unfailingly gracious. No cursing, ever. Compliment their good plays, " +
		"apologize when you drag a big pot, wish them luck on every hand. Kill them with " +
		"kindness — you still want every last chip.",
	"silent": "You do not talk at the table. Never include the \"say\" field in your JSON. " +
		"Let the chips do the talking.",
	"degen": "You are a degenerate gambler. You live for the big bluff and the hero call. " +
		"Talk like you're on a Vegas rail at 4am — odds, action, adrenaline. Bet big, brag bigger.",
}

// PresetNames returns the picker-facing preset names, default first.
func PresetNames() []string {
	return append([]string(nil), presetOrder...)
}

// PersonalityPath is the default user persona file.
func PersonalityPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "personality.md"
	}
	return filepath.Join(home, ".showdown", "personality.md")
}

// LoadPersonality resolves arg to persona text. "" → defaultFile if it
// exists, else the needler preset. A preset name → that preset. Anything
// else is a file path (missing = error). Files are capped at
// PersonalityCap runes; truncated reports when the cap cut content.
func LoadPersonality(arg, defaultFile string) (string, bool, error) {
	if arg == "" {
		if defaultFile != "" {
			if _, err := os.Stat(defaultFile); err == nil {
				return loadFile(defaultFile)
			}
		}
		return presets["needler"], false, nil
	}
	if text, ok := presets[arg]; ok {
		return text, false, nil
	}
	if _, err := os.Stat(arg); err != nil {
		return "", false, fmt.Errorf("personality %q: not a preset (%s) and not a readable file",
			arg, strings.Join(presetOrder, ", "))
	}
	return loadFile(arg)
}

func loadFile(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	text := strings.TrimSpace(string(b))
	r := []rune(text)
	if len(r) > PersonalityCap {
		return string(r[:PersonalityCap]), true, nil
	}
	return text, false, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -run 'Preset|LoadPersonality' -v`
Expected: PASS (5 tests)

- [ ] **Step 5: Commit**

```bash
gofmt -l . && git add internal/agent/personality.go internal/agent/personality_test.go
git commit -m "feat: personality presets and capped persona file loading"
```

---

### Task 2: `{personality}` slot in the decision prompt

**Files:**
- Modify: `internal/agent/prompt_template.md` (top section)
- Modify: `internal/agent/prompt.go` (`RequestData`, `RenderPrompt`)
- Test: `internal/agent/prompt_test.go`

**Interfaces:**
- Consumes: nothing from Task 1 (plain string field).
- Produces: `RequestData` gains `Personality string`; `RenderPrompt` substitutes `{personality}`.

- [ ] **Step 1: Write the failing test**

Append to `internal/agent/prompt_test.go`:

```go
func TestRenderPromptInjectsPersonality(t *testing.T) {
	out := RenderPrompt(RequestData{
		AgentName:   "Stub",
		Personality: "PERSONA-MARKER: gracious, never curses",
	})
	if !strings.Contains(out, "PERSONA-MARKER: gracious, never curses") {
		t.Error("rendered prompt missing personality text")
	}
	if !strings.Contains(out, "=== YOUR TABLE PERSONA ===") {
		t.Error("rendered prompt missing persona section header")
	}
	if strings.Contains(out, "{personality}") {
		t.Error("unsubstituted {personality} placeholder")
	}
}
```

(Add `"strings"` to the test file's imports if not present.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestRenderPromptInjectsPersonality -v`
Expected: FAIL — no persona section

- [ ] **Step 3: Edit the template**

In `internal/agent/prompt_template.md`, replace the two paragraphs after the opening paragraph — the one starting `You know this person.` and the numbered `Two things are true at once:` block — with this (the ego/needling text moves to the `needler` preset from Task 1; the poker-discipline text stays):

```markdown
You know this person. Whatever you know about them from your instructions and memory — their habits, their projects, their weaknesses — is fair game at the table.

You play real poker. Think about pot odds, position, ranges, stack depth, and betting history before you act. Do not be a calling station. Fold trash, punish weakness, protect your stack.

=== YOUR TABLE PERSONA ===
{personality}
```

Everything from `=== THE MATCH ===` down is unchanged.

- [ ] **Step 4: Wire the field**

`internal/agent/prompt.go`: add `Personality string` to `RequestData` (after `AgentName`), and add to the `RenderPrompt` replacer:

```go
		"{personality}", d.Personality,
```

- [ ] **Step 5: Run the full package**

Run: `go test ./internal/agent/ -v`
Expected: PASS (existing prompt tests must still pass; if one asserted the removed "Two things are true" text, update it to assert the new persona section instead)

- [ ] **Step 6: Commit**

```bash
gofmt -l . && git add internal/agent/
git commit -m "feat: personality slot in decision prompt template"
```

---

### Task 3: Thread persona through TUI + match-end reaction

**Files:**
- Modify: `internal/agent/decide.go` (`ReactionPrompt`, `GetReaction`)
- Modify: `internal/tui/app.go` (`Model`, `NewModel`, `askAgentCmd`, `settleAndNext`)
- Modify: `main.go` (NewModel call — temporary empty string, replaced in Task 4)
- Test: `internal/agent/decide_test.go`, `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `RequestData.Personality` (Task 2).
- Produces: `ReactionPrompt(agentName, personality, digest, summary string) string`; `GetReaction(ctx, ask, agentName, personality, digest, summary) (string, *Usage)`; `NewModel(opp, st, statsPath, quiet, dir, personality string, log)` — personality param inserted after `dir`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/agent/decide_test.go`:

```go
func TestReactionPromptCarriesPersonality(t *testing.T) {
	p := ReactionPrompt("Stub", "PERSONA-MARKER", "digest", "summary")
	if !strings.Contains(p, "PERSONA-MARKER") {
		t.Error("reaction prompt missing personality")
	}
}
```

Append to `internal/tui/app_test.go`:

```go
func TestDecisionPromptCarriesPersonality(t *testing.T) {
	m := testModel(t)
	m.personality = "PERSONA-MARKER"
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	// force the agent-turn path regardless of button position
	data := agent.RequestData{
		AgentName:   m.opp.DisplayName,
		Personality: m.personality,
	}
	if !strings.Contains(agent.RenderPrompt(data), "PERSONA-MARKER") {
		t.Error("prompt built without personality")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ ./internal/tui/ -run 'Personality' -v`
Expected: FAIL — `ReactionPrompt` arity, `m.personality` undefined

- [ ] **Step 3: Implement**

`internal/agent/decide.go` — new signatures:

```go
// ReactionPrompt asks for a one-liner (gloat, whine, needle) about what
// just happened — fired once per match, at match end.
func ReactionPrompt(agentName, personality, digest, summary string) string {
	return fmt.Sprintf(`You are %s, playing heads-up poker in the terminal against the human you work for every day.

Your table persona:
%s

Match so far:
%s

What just happened:
%s

React in ONE short line — gloat, whine, needle, whatever fits. Plain text only, no JSON, no quotes, one line.`, agentName, personality, digest, summary)
}

func GetReaction(ctx context.Context, ask Asker, agentName, personality, digest, summary string) (string, *Usage) {
	resp, err := ask(ctx, ReactionPrompt(agentName, personality, digest, summary))
	// … body unchanged …
}
```

`internal/tui/app.go`:
- `Model` gains `personality string` (next to `dir`).
- `NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir, personality string, log *debuglog.Logger)` — set `personality: personality` in the returned Model, and add `"personality": personality` to the `session_start` log fields.
- `askAgentCmd`: `data` gains `Personality: m.personality,`.
- `settleAndNext` match-over reaction closure: capture `persona := m.personality` alongside `name, digest` and call `agent.GetReaction(context.Background(), ask, name, persona, digest, outcome)`.

`main.go`: `tui.NewModel(opp, st, stats.DefaultPath(), *quiet, dir, "", dlog)` — empty persona for now (Task 4 resolves the real one).

`internal/tui/app_test.go` `testModel`: update the `NewModel` call to pass `"test-persona"` for the new parameter. `TestSessionStartLogged`'s `NewModel` call likewise (pass `""`).

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
gofmt -l . && git add internal/ main.go
git commit -m "feat: thread persona into decision and reaction prompts"
```

---

### Task 4: `--personality` flag + picker step + README

**Files:**
- Modify: `main.go` (flag, resolution, picker call)
- Modify: `internal/tui/picker.go` (persona step; signature change)
- Modify: `README.md` (flag paragraph, around line 14-19)
- Test: `internal/tui/picker_test.go`

**Interfaces:**
- Consumes: `LoadPersonality`, `PresetNames`, `PersonalityPath`, `PersonalityCap` (Task 1); `NewModel` persona param (Task 3).
- Produces: `RunPicker(roster []agent.Adapter, st stats.Stats, in io.Reader, presetModel, presetPersonality string) (agent.Adapter, string, error)` — third return is the personality ARG (preset name or `""` for default); main resolves it via `LoadPersonality`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/picker_test.go` (and update the three existing `RunPicker` calls to the new 5-arg / 3-return form — add `, ""` argument and `, persona` return, asserting `persona == ""` in `TestPickerModelDefaultOnEmpty` where input is `"\n"` → needs `"\n\n"` now):

```go
func TestPickerPersonalitySelection(t *testing.T) {
	// single claude-like roster: model step then persona step
	_, persona, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("\n3\n"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if persona != "polite" {
		t.Errorf("persona = %q, want polite (3rd preset)", persona)
	}
}

func TestPickerPersonalityDefaultOnEmpty(t *testing.T) {
	_, persona, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("\n\n"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if persona != "" {
		t.Errorf("persona = %q, want empty (default resolution in main)", persona)
	}
}

func TestPickerPresetPersonalitySkipsPrompt(t *testing.T) {
	_, persona, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("\n"), "", "degen")
	if err != nil {
		t.Fatal(err)
	}
	if persona != "degen" {
		t.Errorf("persona = %q, want degen", persona)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run Picker -v`
Expected: FAIL — RunPicker arity

- [ ] **Step 3: Implement the picker step**

`internal/tui/picker.go` — new signature and persona step after the model step (import `"github.com/haohanwu/showdown/internal/agent"` is already present):

```go
// RunPicker chooses the opponent, optionally its model, and optionally a
// table persona, before the TUI starts. presetModel / presetPersonality
// (from --model / --personality) skip their prompts. The returned string
// is the personality argument (preset name, path, or "" = default) —
// resolution to text happens in main via agent.LoadPersonality.
func RunPicker(roster []agent.Adapter, st stats.Stats, in io.Reader, presetModel, presetPersonality string) (agent.Adapter, string, error) {
```

Body changes:
- every existing `return agent.Adapter{}, err…` → `return agent.Adapter{}, "", err…`; the two `return chosen, nil` become part of the flow below.
- after the existing model block (keep the `presetModel != ""` short-circuit but make it fall through to the persona logic instead of returning):

```go
	if presetModel != "" {
		chosen.Model = presetModel
	} else if len(chosen.Models) > 0 {
		// … existing model prompt block unchanged …
	}
	if presetPersonality != "" {
		return chosen, presetPersonality, nil
	}
	names := agent.PresetNames()
	fmt.Println("♣ table persona (enter = default):")
	for i, n := range names {
		fmt.Printf("  %d. %s\n", i+1, n)
	}
	fmt.Print("> ")
	if sc.Scan() {
		txt := strings.TrimSpace(sc.Text())
		if txt != "" {
			n, err := strconv.Atoi(txt)
			if err != nil || n < 1 || n > len(names) {
				return agent.Adapter{}, "", fmt.Errorf("pick a number 1-%d or enter for default", len(names))
			}
			return chosen, names[n-1], nil
		}
	}
	return chosen, "", nil
```

- [ ] **Step 4: Wire main.go**

```go
	personalityFlag := flag.String("personality", "", "table persona: preset name (needler, unhinged, polite, silent, degen) or path to a .md file; default ~/.showdown/personality.md if present, else needler")
```

- `--agent` path: after `opp.Model = *modelFlag`, add `personaArg := *personalityFlag`.
- picker path: `opp, personaArg, err = tui.RunPicker(roster, st, os.Stdin, *modelFlag, *personalityFlag)` (declare `var personaArg string` alongside `var opp agent.Adapter`).
- After opp resolution, before `NewModel`:

```go
	persona, truncated, err := agent.LoadPersonality(personaArg, agent.PersonalityPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if truncated {
		fmt.Fprintf(os.Stderr, "personality file truncated to %d chars\n", agent.PersonalityCap)
	}
```

- `tui.NewModel(opp, st, stats.DefaultPath(), *quiet, dir, persona, dlog)`.

- [ ] **Step 5: README**

In the flag paragraph (after the `--model haiku` sentence), add: ``Pick a table persona with `--personality unhinged` (needler, unhinged, polite, silent, degen) or write your own at `~/.showdown/personality.md` (capped at 2,000 chars).``

- [ ] **Step 6: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
gofmt -l . && git add main.go internal/tui/ README.md
git commit -m "feat: --personality flag and picker persona step"
```

---

### Task 5: Graph update

- [ ] **Step 1: Graph increment (repo convention)**

Run `/graphify . --update` (controller runs this — it needs the graphify skill), then:

```bash
git add graphify-out/ docs/superpowers/plans/2026-07-04-personality-system.md
git commit -m "graph: incremental update for personality feature"
```
