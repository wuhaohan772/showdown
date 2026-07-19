package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

// Menu rows, top to bottom.
const (
	rowOpponent = iota
	rowModel
	rowPersona
	rowStack
	rowBlind
	rowHands
	rowTalk
	rowCount
)

const (
	menuDefaultModel  = "(default)"
	menuCustomPersona = "custom"

	// DefaultStartStack, DefaultStartSB, and DefaultHandLimit are the
	// sit-and-go defaults. main uses these as --stack/--blind/--hands flag
	// defaults, so "no flag passed" and "flag passed with today's default"
	// land on the same preset row.
	DefaultStartStack = 1500
	DefaultStartSB    = 10
	DefaultHandLimit  = 0 // unlimited: play until someone busts
)

var defaultStackOpts = []int{500, 1000, 1500, 2000, 3000, 5000}
var defaultBlindOpts = []int{5, 10, 25, 50}     // small-blind presets; big blind is always 2x
var defaultHandOpts = []int{0, 10, 25, 50, 100} // 0 = unlimited (bust-only)

// menuModel is the pre-game dashboard: pick opponent/model/persona/stack/
// blind/hands/talk on one screen, enter to start. Replaces the old
// line-based RunPicker.
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

	stackOpts []int
	stackIdx  int

	blindOpts []int // small-blind presets; big blind is always 2x
	blindIdx  int

	handOpts []int // hand-limit presets; 0 = unlimited
	handIdx  int

	talkOn  bool
	entered bool
	quitted bool

	// Lobby motion state (spec 2026-07-19-menu-lobby-design). motion=false
	// (SHOWDOWN_REDUCE_MOTION=1) renders the final state with zero ticks.
	motion bool
	frames int       // monotonic frame counter driving ambient phases
	width  int       // last tea.WindowSizeMsg; 0 = never sized
	deal   cardSlide // entry deal: opponent's two backs, then your A♠ A♥
	statW  int       // tweened lifetime wins for the focused opponent
	statL  int       // tweened lifetime losses
	statC  int       // tweened lifetime cost in cents
}

// newMenuModel builds the dashboard state. personaFile gates the "custom"
// persona entry (callers pass agent.PersonalityPath(); tests a temp path).
func newMenuModel(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetStack, presetSB, presetHands int, presetQuiet bool, personaFile string) menuModel {
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

	m.stackOpts = append([]int{}, defaultStackOpts...)
	if idx := indexOfInt(m.stackOpts, presetStack); idx >= 0 {
		m.stackIdx = idx
	} else {
		m.stackOpts = append(m.stackOpts, presetStack)
		m.stackIdx = len(m.stackOpts) - 1
	}

	m.blindOpts = append([]int{}, defaultBlindOpts...)
	if idx := indexOfInt(m.blindOpts, presetSB); idx >= 0 {
		m.blindIdx = idx
	} else {
		m.blindOpts = append(m.blindOpts, presetSB)
		m.blindIdx = len(m.blindOpts) - 1
	}

	m.handOpts = append([]int{}, defaultHandOpts...)
	if idx := indexOfInt(m.handOpts, presetHands); idx >= 0 {
		m.handIdx = idx
	} else {
		m.handOpts = append(m.handOpts, presetHands)
		m.handIdx = len(m.handOpts) - 1
	}

	m.motion = os.Getenv("SHOWDOWN_REDUCE_MOTION") != "1"
	if m.motion {
		m.deal = newCardSlide(0, 4)
	}

	return m
}

func indexOfInt(s []int, v int) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

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

var menuDim = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

// handsLabel renders a hand-limit preset: 0 is "unlimited", otherwise the
// bare hand count.
func handsLabel(n int) string {
	if n == 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", n)
}

func (m menuModel) View() string {
	talk := "on"
	if !m.talkOn {
		talk = "off"
	}
	sb := m.blindOpts[m.blindIdx]
	vals := [rowCount]string{
		rowOpponent: m.roster[m.oppIdx].DisplayName,
		rowModel:    m.modelOpts[m.oppIdx][m.modelIdx],
		rowPersona:  m.personaOpts[m.personaIdx],
		rowStack:    fmt.Sprintf("%d", m.stackOpts[m.stackIdx]),
		rowBlind:    fmt.Sprintf("%d/%d", sb, sb*2),
		rowHands:    handsLabel(m.handOpts[m.handIdx]),
		rowTalk:     talk,
	}
	labels := [rowCount]string{"opponent", "model", "persona", "stack", "blind", "hands", "table talk"}

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
	case rowStack:
		m.stackIdx = wrap(m.stackIdx, len(m.stackOpts))
	case rowBlind:
		m.blindIdx = wrap(m.blindIdx, len(m.blindOpts))
	case rowHands:
		m.handIdx = wrap(m.handIdx, len(m.handOpts))
	case rowTalk:
		m.talkOn = !m.talkOn
	}
}

// result maps menu selections back to the values main expects: "(default)"
// model → "", "custom" persona → "" (LoadPersonality's file-first default).
func (m menuModel) result() (agent.Adapter, string, bool, int, int, int) {
	ad := m.roster[m.oppIdx]
	if sel := m.modelOpts[m.oppIdx][m.modelIdx]; sel != menuDefaultModel {
		ad.Model = sel
	}
	persona := m.personaOpts[m.personaIdx]
	if persona == menuCustomPersona {
		persona = ""
	}
	return ad, persona, !m.talkOn, m.stackOpts[m.stackIdx], m.blindOpts[m.blindIdx], m.handOpts[m.handIdx]
}

// ErrMenuQuit reports the user backed out of the menu; main exits 0 silently.
var ErrMenuQuit = errors.New("menu quit")

// RunMenu shows the pre-game dashboard in the alt screen (same mode the
// game runs in, so menu → game is one seamless full-screen session) and
// blocks until the user starts a match or quits. Preset args come from
// --model, --personality, --stack, --blind, --hands, --quiet and pre-fill
// their rows. Returns (adapter, persona, quiet, startStack, startSmallBlind,
// handLimit, err).
func RunMenu(roster []agent.Adapter, st stats.Stats, presetModel, presetPersonality string, presetStack, presetSB, presetHands int, presetQuiet bool) (agent.Adapter, string, bool, int, int, int, error) {
	if len(roster) == 0 {
		return agent.Adapter{}, "", false, 0, 0, 0, errors.New("no agent CLIs found on PATH (looked for: claude, codex, gemini)")
	}
	m := newMenuModel(roster, st, presetModel, presetPersonality, presetStack, presetSB, presetHands, presetQuiet, agent.PersonalityPath())
	out, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return agent.Adapter{}, "", false, 0, 0, 0, err
	}
	final := out.(menuModel)
	if final.quitted {
		return agent.Adapter{}, "", false, 0, 0, 0, ErrMenuQuit
	}
	ad, persona, quiet, stack, sb, hands := final.result()
	return ad, persona, quiet, stack, sb, hands, nil
}
