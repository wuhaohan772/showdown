# Menu Screen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the three sequential stdin prompts of `tui.RunPicker` with a single-screen bubbletea dashboard (opponent / model / persona / table-talk rows + career stats line), per `docs/superpowers/specs/2026-07-04-menu-screen-design.md`.

**Architecture:** New `internal/tui/menu.go` holds a self-contained `menuModel` (bubbletea Model) plus a blocking `RunMenu` entry point that runs it as an inline program (no alt-screen) and returns `(agent.Adapter, personaArg string, quiet bool, error)`. `main.go` swaps `RunPicker` for `RunMenu`; the `--agent` fast path is untouched. `picker.go`/`picker_test.go` are deleted.

**Tech Stack:** Go, bubbletea (`tea`), lipgloss (already deps). Tests are plain `go test` unit tests driving `Update`/`View` directly — same pattern as `app_test.go`, no teatest.

## Global Constraints

- Persona-argument semantics unchanged: the string returned by the menu is resolved later by `agent.LoadPersonality` in main; `""` means "user file if present, else needler".
- Empty-roster error string exactly: `no agent CLIs found on PATH (looked for: claude, codex, gemini)`.
- Menu quit (q / ctrl+c) must exit the process with status 0 and print nothing.
- Test helpers `key(s string)` (app_test.go:25) and `stripANSI` (render_test.go:18) already exist in package `tui` — reuse, do not redefine.
- Run `gofmt -w` on every file you touch before committing.

---

### Task 1: menuModel construction — option lists and prefill

**Files:**
- Create: `internal/tui/menu.go`
- Create: `internal/tui/menu_test.go`

**Interfaces:**
- Consumes: `agent.Adapter{Key, DisplayName, Model, Models}`, `agent.PresetNames() []string`, `stats.Stats`.
- Produces (used by Tasks 2–4):
  - `type menuModel struct` with fields `roster []agent.Adapter`, `st stats.Stats`, `focus int`, `oppIdx int`, `modelOpts [][]string`, `modelInit []int`, `modelIdx int`, `personaOpts []string`, `personaIdx int`, `talkOn bool`, `entered bool`, `quitted bool`.
  - `newMenuModel(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetQuiet bool, personaFile string) menuModel` — `personaFile` is the path whose existence gates the `custom` entry (production callers pass `agent.PersonalityPath()`; tests pass a temp path).
  - Row constants: `rowOpponent`, `rowModel`, `rowPersona`, `rowTalk`, `rowCount` (ints via iota).
  - `menuDefaultModel = "(default)"`, `menuCustomPersona = "custom"` (string consts).

**Behavior being built:**
- `modelOpts[i]` = `["(default)", ...roster[i].Models]`; if `presetModel != ""`: for each adapter, select the matching entry, or append `presetModel` verbatim and select it. `modelInit[i]` records that selection (0 when no flag). `modelIdx` starts at `modelInit[0]`.
- `personaOpts` = `agent.PresetNames()`; if `personaFile` exists on disk, `"custom"` is prepended and initially selected (needler is otherwise index 0 = default-first already). If `presetPersonality != ""`: select the matching entry, or append it verbatim (covers file paths) and select it.
- `talkOn = !presetQuiet`.

- [ ] **Step 1: Write the failing tests**

```go
package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

func menuRoster() []agent.Adapter {
	return []agent.Adapter{
		{Key: "claude", DisplayName: "Claude Code", Models: []string{"haiku", "sonnet", "opus"}},
		{Key: "codex", DisplayName: "Codex"}, // no Models: flag-only adapter
	}
}

// personaFile that does not exist
func noPersonaFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "personality.md")
}

func TestMenuModelOptions(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", false, noPersonaFile(t))
	want := []string{"(default)", "haiku", "sonnet", "opus"}
	if len(m.modelOpts[0]) != 4 {
		t.Fatalf("claude modelOpts = %v, want %v", m.modelOpts[0], want)
	}
	for i, w := range want {
		if m.modelOpts[0][i] != w {
			t.Errorf("modelOpts[0][%d] = %q, want %q", i, m.modelOpts[0][i], w)
		}
	}
	if len(m.modelOpts[1]) != 1 || m.modelOpts[1][0] != menuDefaultModel {
		t.Errorf("codex modelOpts = %v, want [(default)]", m.modelOpts[1])
	}
	if m.modelIdx != 0 || m.talkOn != true {
		t.Errorf("defaults: modelIdx=%d talkOn=%v, want 0 true", m.modelIdx, m.talkOn)
	}
}

func TestMenuModelPrefill(t *testing.T) {
	// flag value in the Models list → selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "sonnet", "", true, noPersonaFile(t))
	if m.modelOpts[0][m.modelIdx] != "sonnet" {
		t.Errorf("selected model = %q, want sonnet", m.modelOpts[0][m.modelIdx])
	}
	if m.talkOn {
		t.Error("presetQuiet=true should start talkOn=false")
	}
	// flag value not in any list → appended verbatim everywhere
	m = newMenuModel(menuRoster(), stats.Stats{}, "gpt-x", "", false, noPersonaFile(t))
	if got := m.modelOpts[0][m.modelInit[0]]; got != "gpt-x" {
		t.Errorf("claude prefill = %q, want gpt-x", got)
	}
	if got := m.modelOpts[1][m.modelInit[1]]; got != "gpt-x" {
		t.Errorf("codex prefill = %q, want gpt-x", got)
	}
}

func TestMenuPersonaOptions(t *testing.T) {
	// no user file: presets only, needler (index 0) selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", false, noPersonaFile(t))
	if m.personaOpts[0] != "needler" || m.personaIdx != 0 {
		t.Errorf("personaOpts[0]=%q idx=%d, want needler 0", m.personaOpts[0], m.personaIdx)
	}
	// user file exists: custom prepended and selected
	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("be weird"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", false, f)
	if m.personaOpts[0] != menuCustomPersona || m.personaIdx != 0 {
		t.Errorf("with file: personaOpts[0]=%q idx=%d, want custom 0", m.personaOpts[0], m.personaIdx)
	}
	// --personality preset name → selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "degen", false, f)
	if m.personaOpts[m.personaIdx] != "degen" {
		t.Errorf("prefill persona = %q, want degen", m.personaOpts[m.personaIdx])
	}
	// --personality path → appended verbatim and selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "/tmp/evil.md", false, f)
	if m.personaOpts[m.personaIdx] != "/tmp/evil.md" {
		t.Errorf("prefill persona = %q, want /tmp/evil.md", m.personaOpts[m.personaIdx])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestMenu' -v`
Expected: compile error — `newMenuModel` undefined.

- [ ] **Step 3: Write the implementation**

```go
package tui

import (
	"os"

	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

// Menu rows, top to bottom.
const (
	rowOpponent = iota
	rowModel
	rowPersona
	rowTalk
	rowCount
)

const (
	menuDefaultModel  = "(default)"
	menuCustomPersona = "custom"
)

// menuModel is the pre-game dashboard: pick opponent/model/persona/talk on
// one screen, enter to start. Replaces the old line-based RunPicker.
type menuModel struct {
	roster []agent.Adapter
	st     stats.Stats

	focus  int
	oppIdx int

	modelOpts [][]string // per-adapter options, "(default)" first
	modelInit []int      // per-adapter initial index (--model prefill)
	modelIdx  int

	personaOpts []string
	personaIdx  int

	talkOn  bool
	entered bool
	quitted bool
}

// newMenuModel builds the dashboard state. personaFile gates the "custom"
// persona entry (callers pass agent.PersonalityPath(); tests a temp path).
func newMenuModel(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetQuiet bool, personaFile string) menuModel {
	m := menuModel{roster: roster, st: st, talkOn: !presetQuiet}

	m.modelOpts = make([][]string, len(roster))
	m.modelInit = make([]int, len(roster))
	for i, a := range roster {
		opts := append([]string{menuDefaultModel}, a.Models...)
		sel := 0
		if presetModel != "" {
			sel = -1
			for j, o := range opts {
				if o == presetModel {
					sel = j
				}
			}
			if sel < 0 {
				opts = append(opts, presetModel)
				sel = len(opts) - 1
			}
		}
		m.modelOpts[i], m.modelInit[i] = opts, sel
	}
	m.modelIdx = m.modelInit[0]

	m.personaOpts = agent.PresetNames()
	if _, err := os.Stat(personaFile); err == nil {
		m.personaOpts = append([]string{menuCustomPersona}, m.personaOpts...)
	}
	if presetPersonality != "" {
		m.personaIdx = -1
		for j, o := range m.personaOpts {
			if o == presetPersonality {
				m.personaIdx = j
			}
		}
		if m.personaIdx < 0 {
			m.personaOpts = append(m.personaOpts, presetPersonality)
			m.personaIdx = len(m.personaOpts) - 1
		}
	}
	return m
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestMenu' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/menu.go internal/tui/menu_test.go
git add internal/tui/menu.go internal/tui/menu_test.go
git commit -m "feat: menu model with option lists and flag prefill"
```

---

### Task 2: menuModel Update — navigation, cycling, enter/quit

**Files:**
- Modify: `internal/tui/menu.go` (append)
- Modify: `internal/tui/menu_test.go` (append)

**Interfaces:**
- Consumes: `menuModel` fields from Task 1; `key(s string) tea.KeyMsg` helper from app_test.go:25.
- Produces (used by Tasks 3–4):
  - `func (m menuModel) Init() tea.Cmd` (returns nil)
  - `func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)` — ↑/k ↓/j move focus (clamped, no wrap); ←/h →/l cycle focused row's value (wrapping); opponent change resets `modelIdx` to `modelInit[oppIdx]`; enter sets `entered` and quits; q/ctrl+c sets `quitted` and quits.
  - `func (m menuModel) result() (agent.Adapter, string, bool)` — adapter with `.Model` set (`""` for `(default)`), persona arg (`""` for `custom`), quiet (`!talkOn`).

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/menu_test.go`:

```go
func TestMenuNavigationAndCycling(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", false, noPersonaFile(t))

	// focus moves down and clamps at the last row
	for i := 0; i < 10; i++ {
		v, _ := m.Update(key("j"))
		m = v.(menuModel)
	}
	if m.focus != rowTalk {
		t.Fatalf("focus = %d, want rowTalk (%d)", m.focus, rowTalk)
	}
	// talk row toggles
	v, _ := m.Update(key("l"))
	m = v.(menuModel)
	if m.talkOn {
		t.Error("cycling talk row should toggle talkOn to false")
	}

	// up clamps at the top
	for i := 0; i < 10; i++ {
		v, _ = m.Update(key("k"))
		m = v.(menuModel)
	}
	if m.focus != rowOpponent {
		t.Fatalf("focus = %d, want rowOpponent", m.focus)
	}
	// opponent cycles with wrap: right twice on a 2-roster wraps to start
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if m.oppIdx != 1 {
		t.Fatalf("oppIdx = %d, want 1", m.oppIdx)
	}
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if m.oppIdx != 0 {
		t.Fatalf("oppIdx = %d, want 0 (wrap)", m.oppIdx)
	}
	// left from 0 wraps to end
	v, _ = m.Update(key("h"))
	m = v.(menuModel)
	if m.oppIdx != 1 {
		t.Fatalf("oppIdx = %d, want 1 (left wrap)", m.oppIdx)
	}
}

func TestMenuModelResetsOnOpponentChange(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", false, noPersonaFile(t))
	// focus model row, cycle to haiku
	v, _ := m.Update(key("j"))
	m = v.(menuModel)
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if got := m.modelOpts[0][m.modelIdx]; got != "haiku" {
		t.Fatalf("selected model = %q, want haiku", got)
	}
	// switch opponent: model resets to that adapter's initial index
	v, _ = m.Update(key("k"))
	m = v.(menuModel)
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if m.modelIdx != m.modelInit[1] {
		t.Errorf("modelIdx = %d, want reset to modelInit[1]=%d", m.modelIdx, m.modelInit[1])
	}
}

func TestMenuEnterAndQuit(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", false, noPersonaFile(t))
	v, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = v.(menuModel)
	if !m.entered || cmd == nil {
		t.Error("enter should set entered and return tea.Quit")
	}

	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", false, noPersonaFile(t))
	v, cmd = m.Update(key("q"))
	m = v.(menuModel)
	if !m.quitted || cmd == nil {
		t.Error("q should set quitted and return tea.Quit")
	}
}

func TestMenuResult(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", false, noPersonaFile(t))
	ad, persona, quiet := m.result()
	if ad.Key != "claude" || ad.Model != "" || persona != "needler" || quiet {
		t.Errorf("defaults: got key=%q model=%q persona=%q quiet=%v", ad.Key, ad.Model, persona, quiet)
	}

	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = newMenuModel(menuRoster(), stats.Stats{}, "sonnet", "", true, f)
	ad, persona, quiet = m.result()
	if ad.Model != "sonnet" || persona != "" || !quiet {
		t.Errorf("got model=%q persona=%q quiet=%v, want sonnet \"\" true", ad.Model, persona, quiet)
	}
}
```

Add `tea "github.com/charmbracelet/bubbletea"` to the test file imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestMenu' -v`
Expected: compile error — `Update`/`result` undefined on menuModel.

- [ ] **Step 3: Write the implementation**

Append to `internal/tui/menu.go` (add `tea "github.com/charmbracelet/bubbletea"` to imports):

```go
func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "up", "k":
		if m.focus > 0 {
			m.focus--
		}
	case "down", "j":
		if m.focus < rowCount-1 {
			m.focus++
		}
	case "left", "h":
		m.cycle(-1)
	case "right", "l":
		m.cycle(1)
	case "enter":
		m.entered = true
		return m, tea.Quit
	case "q", "ctrl+c":
		m.quitted = true
		return m, tea.Quit
	}
	return m, nil
}

// cycle moves the focused row's value by d, wrapping.
func (m *menuModel) cycle(d int) {
	wrap := func(i, n int) int { return ((i+d)%n + n) % n }
	switch m.focus {
	case rowOpponent:
		m.oppIdx = wrap(m.oppIdx, len(m.roster))
		m.modelIdx = m.modelInit[m.oppIdx]
	case rowModel:
		m.modelIdx = wrap(m.modelIdx, len(m.modelOpts[m.oppIdx]))
	case rowPersona:
		m.personaIdx = wrap(m.personaIdx, len(m.personaOpts))
	case rowTalk:
		m.talkOn = !m.talkOn
	}
}

// result maps menu selections back to the values main expects: "(default)"
// model → "", "custom" persona → "" (LoadPersonality's file-first default).
func (m menuModel) result() (agent.Adapter, string, bool) {
	ad := m.roster[m.oppIdx]
	if sel := m.modelOpts[m.oppIdx][m.modelIdx]; sel != menuDefaultModel {
		ad.Model = sel
	}
	persona := m.personaOpts[m.personaIdx]
	if persona == menuCustomPersona {
		persona = ""
	}
	return ad, persona, !m.talkOn
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestMenu' -v`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/menu.go internal/tui/menu_test.go
git add internal/tui/menu.go internal/tui/menu_test.go
git commit -m "feat: menu navigation, cycling, enter/quit, result mapping"
```

---

### Task 3: menuModel View

**Files:**
- Modify: `internal/tui/menu.go` (append)
- Modify: `internal/tui/menu_test.go` (append)

**Interfaces:**
- Consumes: menuModel state (Tasks 1–2); `stats.Stats.Line(key string) string` (stats.go:55); `stripANSI` test helper (render_test.go:18); lipgloss.
- Produces: `func (m menuModel) View() string` rendering the approved mockup:

```
♠ SHOWDOWN

  opponent     ◀ Claude Code ▶      vs claude: 12–8 · $1.42
  model          haiku
  persona        needler
  table talk     on

  [ enter ] deal me in    [ q ] quit
```

Focused row's value wears `◀ ▶`; unfocused values render plain (two extra leading spaces keep columns aligned). Stats line renders on the opponent row only, for the selected opponent. Talk row shows `on`/`off`. Hint line dim (lipgloss color 8, like `cardBack` in render.go).

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/menu_test.go`:

```go
func TestMenuView(t *testing.T) {
	st := stats.Stats{"claude": {Wins: 2, Losses: 1}}
	m := newMenuModel(menuRoster(), st, "", "", false, noPersonaFile(t))
	v := stripANSI(m.View())

	for _, want := range []string{
		"♠ SHOWDOWN",
		"◀ Claude Code ▶", // focused row wears the arrows
		"vs claude: 2–1",  // stats line for the selected opponent
		"needler",
		"on",
		"[ enter ] deal me in",
		"[ q ] quit",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q\n%s", want, v)
		}
	}
	if strings.Contains(v, "◀ (default) ▶") {
		t.Error("unfocused model row should not wear arrows")
	}

	// focus the model row: arrows move
	u, _ := m.Update(key("j"))
	m = u.(menuModel)
	v = stripANSI(m.View())
	if !strings.Contains(v, "◀ (default) ▶") {
		t.Errorf("focused model row should wear arrows\n%s", v)
	}
	if strings.Contains(v, "◀ Claude Code ▶") {
		t.Error("unfocused opponent row should not wear arrows")
	}
}
```

Add `"strings"` to the test file imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestMenuView' -v`
Expected: compile error — `View` undefined on menuModel.

- [ ] **Step 3: Write the implementation**

Append to `internal/tui/menu.go` (add `"fmt"`, `"strings"`, `"github.com/charmbracelet/lipgloss"` to imports):

```go
var menuDim = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

func (m menuModel) View() string {
	talk := "on"
	if !m.talkOn {
		talk = "off"
	}
	vals := [rowCount]string{
		rowOpponent: m.roster[m.oppIdx].DisplayName,
		rowModel:    m.modelOpts[m.oppIdx][m.modelIdx],
		rowPersona:  m.personaOpts[m.personaIdx],
		rowTalk:     talk,
	}
	labels := [rowCount]string{"opponent", "model", "persona", "table talk"}

	var b strings.Builder
	b.WriteString("♠ SHOWDOWN\n\n")
	for r := 0; r < rowCount; r++ {
		val := "  " + vals[r]
		if r == m.focus {
			val = "◀ " + vals[r] + " ▶"
		}
		line := fmt.Sprintf("  %-12s %s", labels[r], val)
		if r == rowOpponent {
			line += menuDim.Render("      " + m.st.Line(m.roster[m.oppIdx].Key))
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + menuDim.Render("  [ enter ] deal me in    [ q ] quit") + "\n")
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestMenu' -v`
Expected: PASS (8 tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/menu.go internal/tui/menu_test.go
git add internal/tui/menu.go internal/tui/menu_test.go
git commit -m "feat: menu dashboard view with focus arrows and stats line"
```

---

### Task 4: RunMenu + main.go wiring; delete RunPicker

**Files:**
- Modify: `internal/tui/menu.go` (append)
- Modify: `internal/tui/menu_test.go` (append)
- Modify: `main.go:60-66` (RunPicker call site) and the quiet threading at `main.go:78`
- Delete: `internal/tui/picker.go`, `internal/tui/picker_test.go`

**Interfaces:**
- Consumes: `menuModel` (Tasks 1–3), `agent.PersonalityPath()` (personality.go:40).
- Produces:
  - `var ErrMenuQuit = errors.New("menu quit")`
  - `func RunMenu(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetQuiet bool) (agent.Adapter, string, bool, error)`

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/menu_test.go`:

```go
func TestRunMenuEmptyRoster(t *testing.T) {
	_, _, _, err := RunMenu(nil, stats.Stats{}, "", "", false)
	if err == nil || !strings.Contains(err.Error(), "no agent CLIs found on PATH") {
		t.Errorf("err = %v, want no-agents error", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run 'TestRunMenu' -v`
Expected: compile error — `RunMenu` undefined.

- [ ] **Step 3: Write RunMenu**

Append to `internal/tui/menu.go` (add `"errors"` to imports):

```go
// ErrMenuQuit reports the user backed out of the menu; main exits 0 silently.
var ErrMenuQuit = errors.New("menu quit")

// RunMenu shows the pre-game dashboard inline (no alt-screen) and blocks
// until the user starts a match or quits. Preset args come from --model,
// --personality, --quiet and pre-fill their rows.
func RunMenu(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetQuiet bool) (agent.Adapter, string, bool, error) {
	if len(roster) == 0 {
		return agent.Adapter{}, "", false, errors.New("no agent CLIs found on PATH (looked for: claude, codex, gemini)")
	}
	m := newMenuModel(roster, st, presetModel, presetPersonality, presetQuiet, agent.PersonalityPath())
	out, err := tea.NewProgram(m).Run()
	if err != nil {
		return agent.Adapter{}, "", false, err
	}
	final := out.(menuModel)
	if final.quitted {
		return agent.Adapter{}, "", false, ErrMenuQuit
	}
	ad, persona, quiet := final.result()
	return ad, persona, quiet, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run 'TestRunMenu' -v`
Expected: PASS.

- [ ] **Step 5: Wire main.go, delete picker**

In `main.go`, replace the else-branch of the `*agentFlag` check (currently lines 60–66):

```go
	var opp agent.Adapter
	var personaArg string
	useQuiet := *quiet
	if *agentFlag != "" {
		// ... existing fast path unchanged ...
	} else {
		opp, personaArg, useQuiet, err = tui.RunMenu(roster, st, *modelFlag, *personalityFlag, *quiet)
		if errors.Is(err, tui.ErrMenuQuit) {
			os.Exit(0)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
```

(Add `"errors"` to main.go imports. Declare `useQuiet` as shown; the fast path keeps `useQuiet = *quiet` from the initializer.)

Then change the `NewModel` call (currently line 78) to pass `useQuiet` instead of `*quiet`:

```go
	m := tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, dlog)
```

Delete the old picker:

```bash
git rm internal/tui/picker.go internal/tui/picker_test.go
```

- [ ] **Step 6: Full test suite + build**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build clean, vet clean, all packages PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/tui/menu.go internal/tui/menu_test.go main.go
git add -A
git commit -m "feat: RunMenu dashboard replaces RunPicker"
```

---

## Manual verification (after all tasks)

`go run . --debug` in a terminal with `claude` on PATH: dashboard renders, arrows cycle, enter starts a match, q exits silently with status 0. `go run . --model sonnet` pre-selects sonnet. Not automatable; eyeball once.
