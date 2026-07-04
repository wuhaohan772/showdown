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
