package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
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
	m := newMenuModel(menuRoster(), stats.Stats{}, "sonnet", "", DefaultStartStack, DefaultStartSB, true, noPersonaFile(t))
	if m.modelOpts[0][m.modelIdx] != "sonnet" {
		t.Errorf("selected model = %q, want sonnet", m.modelOpts[0][m.modelIdx])
	}
	if m.talkOn {
		t.Error("presetQuiet=true should start talkOn=false")
	}
	// flag value not in any list → appended verbatim everywhere
	m = newMenuModel(menuRoster(), stats.Stats{}, "gpt-x", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	if got := m.modelOpts[0][m.modelInit[0]]; got != "gpt-x" {
		t.Errorf("claude prefill = %q, want gpt-x", got)
	}
	if got := m.modelOpts[1][m.modelInit[1]]; got != "gpt-x" {
		t.Errorf("codex prefill = %q, want gpt-x", got)
	}
}

func TestMenuPersonaOptions(t *testing.T) {
	// no user file: presets only, needler (index 0) selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	if m.personaOpts[0] != "needler" || m.personaIdx != 0 {
		t.Errorf("personaOpts[0]=%q idx=%d, want needler 0", m.personaOpts[0], m.personaIdx)
	}
	// user file exists: custom prepended and selected
	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("be weird"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, f)
	if m.personaOpts[0] != menuCustomPersona || m.personaIdx != 0 {
		t.Errorf("with file: personaOpts[0]=%q idx=%d, want custom 0", m.personaOpts[0], m.personaIdx)
	}
	// --personality preset name → selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "degen", DefaultStartStack, DefaultStartSB, false, f)
	if m.personaOpts[m.personaIdx] != "degen" {
		t.Errorf("prefill persona = %q, want degen", m.personaOpts[m.personaIdx])
	}
	// --personality path → appended verbatim and selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "/tmp/evil.md", DefaultStartStack, DefaultStartSB, false, f)
	if m.personaOpts[m.personaIdx] != "/tmp/evil.md" {
		t.Errorf("prefill persona = %q, want /tmp/evil.md", m.personaOpts[m.personaIdx])
	}
}

func TestMenuStackAndBlindOptions(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	if got := m.stackOpts[m.stackIdx]; got != DefaultStartStack {
		t.Errorf("default stack = %d, want %d", got, DefaultStartStack)
	}
	if got := m.blindOpts[m.blindIdx]; got != DefaultStartSB {
		t.Errorf("default small blind = %d, want %d", got, DefaultStartSB)
	}
}

func TestMenuStackAndBlindPrefill(t *testing.T) {
	// values already in the preset lists → selected
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", 3000, 25, false, noPersonaFile(t))
	if got := m.stackOpts[m.stackIdx]; got != 3000 {
		t.Errorf("stack prefill = %d, want 3000", got)
	}
	if got := m.blindOpts[m.blindIdx]; got != 25 {
		t.Errorf("blind prefill = %d, want 25", got)
	}
	// values not in the preset lists → appended verbatim and selected
	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", 1234, 7, false, noPersonaFile(t))
	if got := m.stackOpts[m.stackIdx]; got != 1234 {
		t.Errorf("stack custom prefill = %d, want 1234", got)
	}
	if got := m.blindOpts[m.blindIdx]; got != 7 {
		t.Errorf("blind custom prefill = %d, want 7", got)
	}
}

func TestMenuNavigationAndCycling(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))

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

func TestMenuStackAndBlindCycling(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	// opponent -> model -> persona -> stack
	for i := 0; i < 3; i++ {
		v, _ := m.Update(key("j"))
		m = v.(menuModel)
	}
	if m.focus != rowStack {
		t.Fatalf("focus = %d, want rowStack (%d)", m.focus, rowStack)
	}
	before := m.stackOpts[m.stackIdx]
	v, _ := m.Update(key("l"))
	m = v.(menuModel)
	if m.stackOpts[m.stackIdx] == before {
		t.Error("cycling right on stack row should change the selected stack")
	}

	// stack -> blind
	v, _ = m.Update(key("j"))
	m = v.(menuModel)
	if m.focus != rowBlind {
		t.Fatalf("focus = %d, want rowBlind (%d)", m.focus, rowBlind)
	}
	beforeSB := m.blindOpts[m.blindIdx]
	v, _ = m.Update(key("l"))
	m = v.(menuModel)
	if m.blindOpts[m.blindIdx] == beforeSB {
		t.Error("cycling right on blind row should change the selected small blind")
	}
}

func TestMenuModelResetsOnOpponentChange(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
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
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	v, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = v.(menuModel)
	if !m.entered || cmd == nil {
		t.Error("enter should set entered and return tea.Quit")
	}

	m = newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	v, cmd = m.Update(key("q"))
	m = v.(menuModel)
	if !m.quitted || cmd == nil {
		t.Error("q should set quitted and return tea.Quit")
	}
}

func TestMenuResult(t *testing.T) {
	m := newMenuModel(menuRoster(), stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	ad, persona, quiet, stack, sb := m.result()
	if ad.Key != "claude" || ad.Model != "" || persona != "needler" || quiet || stack != DefaultStartStack || sb != DefaultStartSB {
		t.Errorf("defaults: got key=%q model=%q persona=%q quiet=%v stack=%d sb=%d",
			ad.Key, ad.Model, persona, quiet, stack, sb)
	}

	f := filepath.Join(t.TempDir(), "personality.md")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = newMenuModel(menuRoster(), stats.Stats{}, "sonnet", "", 3000, 25, true, f)
	ad, persona, quiet, stack, sb = m.result()
	if ad.Model != "sonnet" || persona != "" || !quiet || stack != 3000 || sb != 25 {
		t.Errorf("got model=%q persona=%q quiet=%v stack=%d sb=%d, want sonnet \"\" true 3000 25",
			ad.Model, persona, quiet, stack, sb)
	}
}

func TestMenuView(t *testing.T) {
	st := stats.Stats{"claude": {Wins: 2, Losses: 1}}
	m := newMenuModel(menuRoster(), st, "", "", DefaultStartStack, DefaultStartSB, false, noPersonaFile(t))
	v := stripANSI(m.View())

	for _, want := range []string{
		"♠ SHOWDOWN",
		"◀ Claude Code ▶", // focused row wears the arrows
		"vs claude: 2–1",  // stats line for the selected opponent
		"needler",
		"1500",
		"10/20",
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

func TestRunMenuEmptyRoster(t *testing.T) {
	_, _, _, _, _, err := RunMenu(nil, stats.Stats{}, "", "", DefaultStartStack, DefaultStartSB, false)
	if err == nil || !strings.Contains(err.Error(), "no agent CLIs found on PATH") {
		t.Errorf("err = %v, want no-agents error", err)
	}
}
