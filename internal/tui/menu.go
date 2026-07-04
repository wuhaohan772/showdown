package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
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

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) View() string { return "" }

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
