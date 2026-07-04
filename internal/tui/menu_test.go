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
