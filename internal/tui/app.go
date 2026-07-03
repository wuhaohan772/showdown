package tui

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/poker"
	"github.com/haohanwu/showdown/internal/stats"
)

type phase int

const (
	phaseHumanTurn phase = iota
	phaseRaiseInput
	phaseTalkInput
	phaseAgentTurn
	phaseRunout
	phaseHandEnd
	phaseMatchOver
)

const decisionTimeout = 45 * time.Second

type startHandMsg struct{}
type decisionMsg struct {
	act      poker.Action
	say      string
	fallback bool
}
type reactionMsg string
type runoutTickMsg struct{}

type Model struct {
	opp       agent.Adapter
	stats     stats.Stats
	statsPath string
	quiet     bool
	dir       string
	rng       *rand.Rand

	match  *poker.Match
	hand   *poker.Hand
	digest *agent.Digest

	phase    phase
	input    textinput.Model
	spin     spinner.Model
	agentSay string
	banner   string
	revealed int
	saved    bool
}

func NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir string) Model {
	in := textinput.New()
	in.CharLimit = 120
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	return Model{
		opp: opp, stats: st, statsPath: statsPath, quiet: quiet, dir: dir,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
		match: poker.NewMatch(), digest: agent.NewDigest(),
		input: in, spin: sp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, func() tea.Msg { return startHandMsg{} })
}

func (m Model) humanSeat() int { return m.match.SeatOf(0) }
func (m Model) agentSeat() int { return m.match.SeatOf(1) }

func (m *Model) startHand() tea.Cmd {
	sb, bb := m.match.Blinds()
	seatStacks := [2]int{m.match.Stacks[m.match.PlayerAt(0)], m.match.Stacks[m.match.PlayerAt(1)]}
	m.hand = poker.NewHand(seatStacks, sb, bb, m.rng)
	m.agentSay = ""
	m.banner = fmt.Sprintf("hand %d — blinds %d/%d", m.match.HandNum, sb, bb)
	m.revealed = 0
	return m.advance()
}

// advance routes control after any applied action.
func (m *Model) advance() tea.Cmd {
	if m.hand.Result() != nil {
		return m.finishHand()
	}
	if m.hand.Actor == m.humanSeat() {
		m.phase = phaseHumanTurn
		return nil
	}
	m.phase = phaseAgentTurn
	return m.askAgentCmd()
}

func (m *Model) askAgentCmd() tea.Cmd {
	sb, bb := m.match.Blinds()
	data := agent.RequestData{
		AgentName:    m.opp.DisplayName,
		MatchDigest:  m.digest.String(),
		HandState:    agent.BuildHandState(m.hand, m.agentSeat(), sb, bb),
		TalkLog:      m.digest.HandTalk(),
		LegalActions: joinActions(m.hand.LegalActions()),
		MinRaise:     m.hand.MinRaiseTo(),
		MaxAmount:    m.hand.MaxRaiseTo(),
	}
	legal := m.hand.LegalActions()
	ask := m.opp.Asker(m.dir, decisionTimeout)
	return func() tea.Msg {
		act, say, fb := agent.GetDecision(context.Background(), ask, data, legal)
		return decisionMsg{act: act, say: say, fallback: fb}
	}
}

func (m *Model) finishHand() tea.Cmd {
	r := m.hand.Result()
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
		m.banner = fmt.Sprintf("split pot (%d) — %s. enter for next hand", r.Pot, r.Desc)
	} else {
		m.banner = fmt.Sprintf("%s wins %d (%s). enter for next hand", winnerName, r.Pot, how)
	}
	summary := fmt.Sprintf("Hand %d: %s won %d (%s).", m.match.HandNum, winnerName, r.Pot, how)
	m.digest.EndHand(summary)
	m.revealed = len(m.hand.Board)

	var cmds []tea.Cmd
	if !m.quiet {
		ask := m.opp.Asker(m.dir, decisionTimeout)
		name, digest := m.opp.DisplayName, m.digest.String()
		cmds = append(cmds, func() tea.Msg {
			return reactionMsg(agent.GetReaction(context.Background(), ask, name, digest, summary))
		})
	}
	if r.Showdown {
		m.phase = phaseRunout
		m.revealed = 0
		cmds = append(cmds, runoutTick())
	}
	return tea.Batch(cmds...)
}

func runoutTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return runoutTickMsg{} })
}

func (m *Model) settleAndNext() tea.Cmd {
	final := [2]int{m.hand.Seats[0].Stack, m.hand.Seats[1].Stack}
	m.match.NextHand(final)
	if m.match.Over() {
		m.phase = phaseMatchOver
		if !m.saved {
			m.saved = true
			r := m.stats[m.opp.Key]
			if m.match.Winner() == 0 {
				r.Wins++
			} else {
				r.Losses++
			}
			m.stats[m.opp.Key] = r
			_ = m.stats.Save(m.statsPath)
		}
		return nil
	}
	return m.startHand()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startHandMsg:
		return m, m.startHand()
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case decisionMsg:
		if err := m.hand.Apply(msg.act); err != nil {
			// GetDecision guarantees legality; a failure here is a bug — force fallback.
			_ = m.hand.Apply(agent.FallbackAction(m.hand.LegalActions()))
		}
		if msg.say != "" && !m.quiet {
			m.agentSay = msg.say
			m.digest.AddTalk(m.opp.DisplayName, msg.say)
		}
		if msg.fallback {
			m.agentSay = "(agent glitched — forced " + string(msg.act.Type) + ")"
		}
		return m, m.advance()
	case reactionMsg:
		if string(msg) != "" && !m.quiet {
			m.agentSay = string(msg)
		}
		return m, nil
	case runoutTickMsg:
		if m.phase == phaseRunout {
			m.revealed++
			if m.revealed >= len(m.hand.Board) {
				m.revealed = len(m.hand.Board)
				m.phase = phaseHandEnd
				return m, nil
			}
			return m, runoutTick()
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" || (k == "q" && m.phase != phaseTalkInput && m.phase != phaseRaiseInput) {
		return m, tea.Quit
	}
	switch m.phase {
	case phaseRaiseInput, phaseTalkInput:
		switch k {
		case "enter":
			return m.confirmInput()
		case "esc":
			m.phase = phaseHumanTurn
			m.input.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	case phaseHumanTurn:
		return m.humanAction(k)
	case phaseHandEnd:
		if k == "enter" {
			return m, m.settleAndNext()
		}
	case phaseMatchOver:
		if k == "enter" || k == "q" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) humanAction(k string) (tea.Model, tea.Cmd) {
	legal := map[poker.ActionType]bool{}
	for _, a := range m.hand.LegalActions() {
		legal[a] = true
	}
	switch k {
	case "f":
		if legal[poker.Fold] {
			_ = m.hand.Apply(poker.Action{Type: poker.Fold})
			return m, m.advance()
		}
	case "c":
		if legal[poker.Call] {
			_ = m.hand.Apply(poker.Action{Type: poker.Call})
			return m, m.advance()
		}
		if legal[poker.Check] {
			_ = m.hand.Apply(poker.Action{Type: poker.Check})
			return m, m.advance()
		}
	case "r", "b":
		if legal[poker.Bet] || legal[poker.Raise] {
			m.phase = phaseRaiseInput
			m.input.Placeholder = fmt.Sprintf("raise to (%d-%d)", m.hand.MinRaiseTo(), m.hand.MaxRaiseTo())
			m.input.SetValue("")
			m.input.Focus()
		}
	case "a":
		if legal[poker.Bet] || legal[poker.Raise] {
			t := poker.Raise
			if legal[poker.Bet] {
				t = poker.Bet
			}
			_ = m.hand.Apply(poker.Action{Type: t, To: m.hand.MaxRaiseTo()})
			return m, m.advance()
		}
	case "t":
		m.phase = phaseTalkInput
		m.input.Placeholder = "talk trash"
		m.input.SetValue("")
		m.input.Focus()
	}
	return m, nil
}

func (m Model) confirmInput() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.input.Value())
	m.input.Blur()
	if m.phase == phaseTalkInput {
		if val != "" {
			m.digest.AddTalk("HUMAN", val)
		}
		m.phase = phaseHumanTurn
		return m, nil
	}
	// raise input
	n, err := strconv.Atoi(val)
	if err != nil {
		m.banner = "enter a number"
		m.phase = phaseHumanTurn
		return m, nil
	}
	t := poker.Raise
	for _, a := range m.hand.LegalActions() {
		if a == poker.Bet {
			t = poker.Bet
		}
	}
	if err := m.hand.Apply(poker.Action{Type: t, To: n}); err != nil {
		m.banner = err.Error()
		m.phase = phaseHumanTurn
		return m, nil
	}
	return m, m.advance()
}

func joinActions(as []poker.ActionType) string {
	strs := make([]string, len(as))
	for i, a := range as {
		strs[i] = string(a)
	}
	return strings.Join(strs, ", ")
}

var (
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	sayStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Italic(true)
	bannerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
)

func (m Model) View() string {
	if m.hand == nil {
		return "shuffling..."
	}
	var b strings.Builder
	hs, as := m.humanSeat(), m.agentSeat()
	agentHoleUp := m.phase == phaseMatchOver ||
		(m.hand.Result() != nil && m.hand.Result().Showdown && m.phase != phaseRunout)

	fmt.Fprintf(&b, "  ♠ %s   stack: %d\n", m.opp.DisplayName, m.hand.Seats[as].Stack)
	rev := 0
	if agentHoleUp {
		rev = 2
	}
	b.WriteString(indent(RenderCardRow(m.hand.Hole[as][:], rev), 2) + "\n")
	if m.agentSay != "" && !m.quiet {
		b.WriteString(sayStyle.Render(`  "`+m.agentSay+`"`) + "\n")
	}
	if m.phase == phaseAgentTurn {
		b.WriteString(dimStyle.Render("  "+m.spin.View()+m.opp.DisplayName+" is thinking...") + "\n")
	}
	b.WriteString("\n")

	boardRev := len(m.hand.Board)
	if m.phase == phaseRunout {
		boardRev = m.revealed
	}
	if len(m.hand.Board) > 0 {
		b.WriteString(indent(RenderCardRow(m.hand.Board, boardRev), 2) + "\n")
	}
	fmt.Fprintf(&b, "  pot: %d\n\n", m.hand.Pot)

	b.WriteString(indent(RenderCardRow(m.hand.Hole[hs][:], 2), 2) + "\n")
	fmt.Fprintf(&b, "  ♥ YOU   stack: %d\n\n", m.hand.Seats[hs].Stack)

	b.WriteString(bannerStyle.Render("  "+m.banner) + "\n")
	switch m.phase {
	case phaseHumanTurn:
		opts := []string{}
		for _, a := range m.hand.LegalActions() {
			switch a {
			case poker.Fold:
				opts = append(opts, "(f)old")
			case poker.Check:
				opts = append(opts, "(c)heck")
			case poker.Call:
				opts = append(opts, fmt.Sprintf("(c)all %d", m.hand.CallAmount()))
			case poker.Bet:
				opts = append(opts, "(b)et", "(a)ll-in")
			case poker.Raise:
				opts = append(opts, "(r)aise", "(a)ll-in")
			}
		}
		opts = append(opts, "(t)alk")
		b.WriteString("  > " + strings.Join(opts, "  ") + "\n")
	case phaseRaiseInput, phaseTalkInput:
		b.WriteString("  > " + m.input.View() + "\n")
	case phaseMatchOver:
		winner := "YOU WIN THE MATCH"
		if m.match.Winner() == 1 {
			winner = strings.ToUpper(m.opp.DisplayName) + " WINS THE MATCH"
		}
		b.WriteString("\n  ═══ " + winner + " ═══\n  " + m.stats.Line(m.opp.Key) + "\n  enter/q to exit\n")
	}
	return b.String()
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}
