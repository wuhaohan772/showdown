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
	"github.com/haohanwu/showdown/internal/debuglog"
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
	usage    *agent.Usage
}
type reactionMsg struct {
	say   string
	usage *agent.Usage
}
type runoutTickMsg struct{}

// chatLine is one entry in the UI-side talk feed. The feed is a pure display
// concern, parallel to the agent Digest (which keeps feeding the agent).
type chatLine struct {
	who     string // opponent DisplayName, "you", or "" for a divider
	text    string // spoken text, or "hand N" for a divider
	divider bool
}

type Model struct {
	opp         agent.Adapter
	stats       stats.Stats
	statsPath   string
	quiet       bool
	dir         string
	personality string
	rng         *rand.Rand
	log         *debuglog.Logger

	match  *poker.Match
	hand   *poker.Hand
	digest *agent.Digest

	phase        phase
	talkReturn   phase // phase to restore when talk input closes
	input        textinput.Model
	spin         spinner.Model
	banner       string
	revealed     int
	saved        bool
	sessionUsage agent.Usage

	feed          []chatLine
	width, height int

	// reactionPending: the match-end reaction cmd is in flight; the match-over
	// screen waits for it (spinner) and the first q/enter shows a skip hint
	// instead of quitting. skipHinted records that first press.
	reactionPending bool
	skipHinted      bool

	// finalHumanSeat captures humanSeat() just before NextHand() advances
	// HandNum (which flips ButtonPlayer/SeatOf). The match-over screen still
	// renders m.hand (the just-finished hand), so it must keep using the seat
	// mapping that was valid for that hand, not the next one.
	finalHumanSeat int

	// P2 persistent session (claude only; nil = stateless adapter or start
	// failed). pendingResults are digest hand-summary lines not yet conveyed
	// to the session — the next delta turn carries and clears them.
	session        *agent.Session
	pendingResults []string
}

func NewModel(opp agent.Adapter, st stats.Stats, statsPath string, quiet bool, dir, personality string, startStack, startSB, handLimit int, log *debuglog.Logger) Model {
	in := textinput.New()
	in.CharLimit = 120
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	seed := time.Now().UnixNano()
	log.Log("session_start", map[string]any{
		"agent_key": opp.Key, "agent_name": opp.DisplayName,
		"model": opp.Model, "quiet": quiet, "dir": dir, "seed": seed,
		"personality": personality, "start_stack": startStack, "start_sb": startSB, "hand_limit": handLimit,
	})
	return Model{
		opp: opp, stats: st, statsPath: statsPath, quiet: quiet, dir: dir,
		personality: personality, log: log,
		rng:   rand.New(rand.NewSource(seed)),
		match: poker.NewMatch(startStack, startSB, handLimit), digest: agent.NewDigest(startStack),
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
	m.log.Log("hand_start", map[string]any{
		"hand": m.match.HandNum, "sb": sb, "bb": bb,
		"human_seat": m.humanSeat(),
		"hole_seat0": cardStrings(m.hand.Hole[0][:]),
		"hole_seat1": cardStrings(m.hand.Hole[1][:]),
		"stacks":     []int{m.hand.Seats[0].Stack, m.hand.Seats[1].Stack},
	})
	m.feed = append(m.feed, chatLine{text: fmt.Sprintf("hand %d", m.match.HandNum), divider: true})
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
		Personality:  m.personality,
		MatchDigest:  m.digest.String(),
		HandState:    agent.BuildHandState(m.hand, m.agentSeat(), sb, bb),
		TalkLog:      m.digest.HandTalk(),
		LegalActions: joinActions(m.hand.LegalActions()),
		MinRaise:     m.hand.MinRaiseTo(),
		MaxAmount:    m.hand.MaxRaiseTo(),
	}
	legal := m.hand.LegalActions()
	full := agent.RenderPrompt(data)
	prompt := full
	stateless := m.opp.Asker(m.dir, decisionTimeout)
	var ask agent.Asker
	if s := m.ensureSession(); s != nil {
		if s.Primed() {
			prompt = agent.RenderDelta(data, m.pendingResults)
		} else {
			// priming turn: the full render (with digest) IS the catch-up
			s.MarkPrimed()
		}
		m.pendingResults = nil
		// Wrap each leaf asker individually so the transcript records the
		// prompt that was actually sent (Fix 2): the session asker gets the
		// delta and the stateless asker gets the full prompt — wrapping the
		// composite would log the delta even on a stateless fallback.
		wrappedSession := debuglog.WrapAsker(s.Asker(), m.log)
		wrappedStateless := debuglog.WrapAsker(stateless, m.log)
		ask = func(ctx context.Context, p string) (agent.Response, error) {
			if !s.Alive() {
				return wrappedStateless(ctx, full)
			}
			resp, err := wrappedSession(ctx, p)
			if err != nil {
				// session died mid-decision: same decision continues via a
				// cold spawn with the full prompt (ADR-0001 — never stall).
				return wrappedStateless(ctx, full)
			}
			return resp, nil
		}
	} else {
		ask = debuglog.WrapAsker(stateless, m.log)
	}
	return func() tea.Msg {
		act, say, fb, u := agent.GetDecisionPrompt(context.Background(), ask, prompt, data, legal)
		return decisionMsg{act: act, say: say, fallback: fb, usage: u}
	}
}

// ensureSession returns a live session, lazily (re)starting one for
// session-capable adapters. One start attempt per call; on failure the
// caller proceeds stateless and a later decision retries (spec: failure &
// restart policy).
func (m *Model) ensureSession() *agent.Session {
	if m.session.Alive() {
		return m.session
	}
	if !m.opp.SupportsSession() {
		return nil
	}
	s, err := m.opp.StartSession(m.dir, decisionTimeout)
	if err != nil {
		m.log.Log("agent_session_start_failed", map[string]any{"error": err.Error()})
		m.session = nil
		return nil
	}
	m.log.Log("agent_session_start", map[string]any{"restart": m.session != nil})
	m.session = s
	return s
}

// CloseSession releases the opponent process; main calls it after the tea
// program exits. Nil-safe.
func (m Model) CloseSession() { m.session.Close() }

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
		m.banner = fmt.Sprintf("split pot (%d) — %s", r.Pot, r.Desc)
	} else {
		m.banner = fmt.Sprintf("%s wins %d (%s)", winnerName, r.Pot, how)
	}
	summary := agentHandSummary(m.match.HandNum, r, r.Winner == m.agentSeat())
	m.digest.EndHand(summary)
	m.pendingResults = append(m.pendingResults, summary)
	m.revealed = len(m.hand.Board)

	// No per-hand reaction call: each was a full CLI spawn (ADR-0003 cost
	// posture). Table talk flows through the decision's "say" field; the
	// single reaction call fires at match end (settleAndNext).
	if r.Showdown {
		m.phase = phaseRunout
		m.revealed = 0
		return runoutTick()
	}
	return nil
}

// agentHandSummary builds the digest entry for a finished hand from the
// AGENT's perspective — the digest is fed back to the agent, so "you" must
// mean the agent and the folder is named explicitly. (The human-facing
// banner is built separately in finishHand.)
func agentHandSummary(handNum int, r *poker.Result, agentWon bool) string {
	if r.Split {
		return fmt.Sprintf("Hand %d: split pot (%d) — %s.", handNum, r.Pot, r.Desc)
	}
	winner, loser := "the human", "you"
	if agentWon {
		winner, loser = "you", "the human"
	}
	how := loser + " folded"
	if r.Showdown {
		how = r.Desc
	}
	return fmt.Sprintf("Hand %d: %s won %d (%s).", handNum, winner, r.Pot, how)
}

func runoutTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return runoutTickMsg{} })
}

// matchOverOutcome states how the match ended, from the agent's own
// perspective, for its match-over reaction prompt. It must never claim a
// bust that didn't happen (table-truth rule) — a match can also end by
// hitting a configured hand limit with the bigger stack, which is not a
// bust, and the agent's own "memory" of the match must never state
// something false about how it ended.
func matchOverOutcome(endedByBust, agentWon bool) string {
	switch {
	case endedByBust && agentWon:
		return "MATCH OVER: you WON the match. Your human is busted."
	case endedByBust:
		return "MATCH OVER: you LOST the match to your human. They took every chip."
	case agentWon:
		return "MATCH OVER: you WON the match. Hand limit hit — you had the bigger stack."
	default:
		return "MATCH OVER: you LOST the match to your human. Hand limit hit — they had the bigger stack."
	}
}

func (m *Model) settleAndNext() tea.Cmd {
	final := [2]int{m.hand.Seats[0].Stack, m.hand.Seats[1].Stack}
	m.finalHumanSeat = m.humanSeat()
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
			r.TokensIn += int64(m.sessionUsage.TotalIn())
			r.TokensOut += int64(m.sessionUsage.OutputTokens)
			r.CostUSD += m.sessionUsage.CostUSD
			m.stats[m.opp.Key] = r
			_ = m.stats.Save(m.statsPath)
		}
		if !m.quiet {
			outcome := matchOverOutcome(m.match.EndedByBust(), m.match.Winner() == 1)
			name, digest, persona := m.opp.DisplayName, m.digest.String(), m.personality
			fullPrompt := agent.ReactionPrompt(name, persona, digest, outcome)
			prompt := fullPrompt
			stateless := m.opp.Asker(m.dir, decisionTimeout)
			var ask agent.Asker
			if s := m.session; s.Alive() && s.Primed() {
				// the session already knows the match; one short turn does it.
				// Prepend any pending hand summaries not yet seen by the session
				// so it has the final hand context before reacting.
				prompt = strings.Join(append(append([]string{}, m.pendingResults...), outcome), "\n") + "\nReact in ONE short line — gloat, whine, needle, whatever fits. Plain text only, no JSON, no quotes, one line."
				// Wrap each leaf individually (same Fix-2 rationale as askAgentCmd).
				wrappedSession := debuglog.WrapAsker(s.Asker(), m.log)
				wrappedStateless := debuglog.WrapAsker(stateless, m.log)
				ask = func(ctx context.Context, p string) (agent.Response, error) {
					resp, err := wrappedSession(ctx, p)
					if err != nil {
						return wrappedStateless(ctx, fullPrompt)
					}
					return resp, nil
				}
			} else {
				ask = debuglog.WrapAsker(stateless, m.log)
			}
			// Late usage from this call is folded into stats by the
			// reactionMsg handler (post-save re-save path).
			m.reactionPending = true
			return func() tea.Msg {
				say, u := agent.GetReactionPrompt(context.Background(), ask, prompt)
				return reactionMsg{say: say, usage: u}
			}
		}
		return nil
	}
	return m.startHand()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startHandMsg:
		cmd := m.startHand()
		return m, cmd
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case decisionMsg:
		m.sessionUsage.Add(msg.usage)
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
			m.digest.AddTalk(m.opp.DisplayName, msg.say)
			m.feed = append(m.feed, chatLine{who: m.opp.DisplayName, text: msg.say})
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
	case reactionMsg:
		m.sessionUsage.Add(msg.usage)
		if msg.usage != nil && m.phase == phaseMatchOver && m.saved {
			r := m.stats[m.opp.Key]
			r.TokensIn += int64(msg.usage.TotalIn())
			r.TokensOut += int64(msg.usage.OutputTokens)
			r.CostUSD += msg.usage.CostUSD
			m.stats[m.opp.Key] = r
			_ = m.stats.Save(m.statsPath)
		}
		if msg.say != "" && !m.quiet {
			m.feed = append(m.feed, chatLine{who: m.opp.DisplayName, text: msg.say})
		}
		m.reactionPending = false
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
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
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	m.log.Log("human_key", map[string]any{"key": k, "phase": phaseName(m.phase)})
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if k == "q" && m.phase != phaseTalkInput && m.phase != phaseRaiseInput {
		if m.phase == phaseMatchOver && m.reactionPending && !m.skipHinted {
			// parting shot still in flight: first q shows the skip hint
			// instead of quitting; q again is the escape hatch.
			m.skipHinted = true
			return m, nil
		}
		return m, tea.Quit
	}
	switch m.phase {
	case phaseRaiseInput, phaseTalkInput:
		switch k {
		case "enter":
			return m.confirmInput()
		case "esc":
			if m.phase == phaseTalkInput {
				m.phase = m.talkReturn
			} else {
				m.phase = phaseHumanTurn
			}
			m.input.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	case phaseHumanTurn:
		return m.humanAction(k)
	case phaseAgentTurn:
		if k == "t" {
			m.openTalk()
			return m, nil
		}
	case phaseHandEnd:
		if k == "t" {
			m.openTalk()
			return m, nil
		}
		if k == "enter" {
			cmd := m.settleAndNext()
			return m, cmd
		}
	case phaseMatchOver:
		if k == "enter" {
			if m.reactionPending && !m.skipHinted {
				m.skipHinted = true
				return m, nil
			}
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
			_ = m.apply(poker.Action{Type: poker.Fold})
			cmd := m.advance()
			return m, cmd
		}
	case "c":
		if legal[poker.Call] {
			_ = m.apply(poker.Action{Type: poker.Call})
			cmd := m.advance()
			return m, cmd
		}
		if legal[poker.Check] {
			_ = m.apply(poker.Action{Type: poker.Check})
			cmd := m.advance()
			return m, cmd
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
			_ = m.apply(poker.Action{Type: t, To: m.hand.MaxRaiseTo()})
			cmd := m.advance()
			return m, cmd
		}
	case "t":
		m.openTalk()
	}
	return m, nil
}

// openTalk switches to talk input, remembering which phase to restore
// when the input closes (enter or esc).
func (m *Model) openTalk() {
	m.talkReturn = m.phase
	m.phase = phaseTalkInput
	m.input.Placeholder = "talk trash"
	m.input.SetValue("")
	m.input.Focus()
}

func (m Model) confirmInput() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.input.Value())
	m.input.Blur()
	if m.phase == phaseTalkInput {
		if val != "" {
			m.digest.AddTalk("HUMAN", val)
			m.feed = append(m.feed, chatLine{who: "you", text: val})
		}
		m.phase = m.talkReturn
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
	if err := m.apply(poker.Action{Type: t, To: n}); err != nil {
		m.banner = err.Error()
		m.phase = phaseHumanTurn
		return m, nil
	}
	cmd := m.advance()
	return m, cmd
}

func cardStrings(cs []poker.Card) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.String()
	}
	return out
}

func phaseName(p phase) string {
	switch p {
	case phaseHumanTurn:
		return "human_turn"
	case phaseRaiseInput:
		return "raise_input"
	case phaseTalkInput:
		return "talk_input"
	case phaseAgentTurn:
		return "agent_turn"
	case phaseRunout:
		return "runout"
	case phaseHandEnd:
		return "hand_end"
	case phaseMatchOver:
		return "match_over"
	}
	return "unknown"
}

// apply funnels every Hand.Apply so the debug log records each action and
// the resulting engine state. Behavior is identical to calling
// m.hand.Apply directly.
func (m *Model) apply(a poker.Action) error {
	actor := m.hand.Actor
	err := m.hand.Apply(a)
	f := map[string]any{
		"hand": m.match.HandNum, "actor_seat": actor,
		"action": string(a.Type), "to": a.To,
		"street":      m.hand.Street.String(),
		"pot":         m.hand.Pot,
		"stacks":      []int{m.hand.Seats[0].Stack, m.hand.Seats[1].Stack},
		"board":       cardStrings(m.hand.Board),
		"current_bet": m.hand.CurrentBet,
		"committed":   []int{m.hand.Seats[0].Committed, m.hand.Seats[1].Committed},
	}
	if err != nil {
		f["error"] = err.Error()
	}
	m.log.Log("apply", f)
	return err
}

func joinActions(as []poker.ActionType) string {
	strs := make([]string, len(as))
	for i, a := range as {
		strs[i] = string(a)
	}
	return strings.Join(strs, ", ")
}

var (
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	sayStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Italic(true)
	humanSayStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	bannerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
)

const (
	chatPanelWidth        = 30 // fixed panel width (decision: not scaled to terminal)
	chatPanelMinTermWidth = 74 // below this (or width unknown) fall back to the bottom log
	chatLogLines          = 4  // last-K lines shown by the narrow fallback
)

func (m Model) View() string {
	if m.hand == nil {
		return "shuffling..."
	}
	table, actions := m.renderTable()
	if m.quiet {
		return table + actions
	}
	if m.width >= chatPanelMinTermWidth {
		return m.composeWithPanel(table + actions)
	}
	return table + m.renderBottomLog() + actions
}

// renderTable draws the left column: the table through the banner, and the
// action bar separately so the narrow fallback can slot the talk log between
// them.
func (m Model) renderTable() (table, actions string) {
	var b strings.Builder
	hs, as := m.humanSeat(), m.agentSeat()
	agentStack, humanStack := m.hand.Seats[as].Stack, m.hand.Seats[hs].Stack
	if m.phase == phaseMatchOver {
		// m.hand is the just-finished hand; HandNum has already advanced (via
		// NextHand), which flips humanSeat()/agentSeat(). Use the seat mapping
		// captured before that advance, and read stacks from the match (which
		// is player-id indexed and unaffected by the seat flip).
		hs = m.finalHumanSeat
		as = 1 - hs
		agentStack, humanStack = m.match.Stacks[1], m.match.Stacks[0]
	}
	inRunout := m.phase == phaseRunout || (m.phase == phaseTalkInput && m.talkReturn == phaseRunout)
	agentHoleUp := m.phase == phaseMatchOver ||
		(m.hand.Result() != nil && m.hand.Result().Showdown && !inRunout)

	fmt.Fprintf(&b, "  ♠ %s   stack: %d\n", m.opp.DisplayName, agentStack)
	rev := 0
	if agentHoleUp {
		rev = 2
	}
	b.WriteString(indent(RenderCardRow(m.hand.Hole[as][:], rev), 2) + "\n")
	if m.phase == phaseAgentTurn || (m.phase == phaseTalkInput && m.talkReturn == phaseAgentTurn) {
		b.WriteString(dimStyle.Render("  "+m.spin.View()+m.opp.DisplayName+" is thinking...") + "\n")
	}
	b.WriteString("\n")

	boardRev := len(m.hand.Board)
	if inRunout {
		boardRev = m.revealed
	}
	if len(m.hand.Board) > 0 {
		b.WriteString(indent(RenderCardRow(m.hand.Board, boardRev), 2) + "\n")
	}
	fmt.Fprintf(&b, "  pot: %d\n\n", m.hand.Pot)

	b.WriteString(indent(RenderCardRow(m.hand.Hole[hs][:], 2), 2) + "\n")
	fmt.Fprintf(&b, "  ♥ YOU   stack: %d\n", humanStack)
	b.WriteString("\n")

	b.WriteString(bannerStyle.Render("  "+m.banner) + "\n")
	table = b.String()

	var a strings.Builder
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
		a.WriteString("  > " + strings.Join(opts, "  ") + "\n")
	case phaseRaiseInput, phaseTalkInput:
		a.WriteString("  > " + m.input.View() + "\n")
	case phaseAgentTurn:
		a.WriteString("  > (t)alk\n")
	case phaseHandEnd:
		a.WriteString("  > (enter) next hand  (t)alk\n")
	case phaseMatchOver:
		winner := "YOU WIN THE MATCH"
		if m.match.Winner() == 1 {
			winner = strings.ToUpper(m.opp.DisplayName) + " WINS THE MATCH"
		}
		usageLine := ""
		if m.sessionUsage != (agent.Usage{}) {
			usageLine = fmt.Sprintf("\n  tokens this match: %d in / %d out · $%.2f",
				m.sessionUsage.TotalIn(), m.sessionUsage.OutputTokens, m.sessionUsage.CostUSD)
		}
		exit := "enter/q to exit"
		if m.reactionPending && m.skipHinted {
			exit = "(waiting for parting shot — q again to skip)"
		}
		a.WriteString("\n  ═══ " + winner + " ═══\n  " + m.stats.Line(m.opp.Key) + usageLine + "\n  " + exit + "\n")
	}
	return table, a.String()
}

// feedLineGroups renders the feed into styled display lines wrapped to width
// w, one group per feed entry (plus the typing spinner while the match-end
// reaction is in flight) so callers can truncate at message boundaries.
func (m Model) feedLineGroups(w int) [][]string {
	var out [][]string
	for _, l := range m.feed {
		if l.divider {
			label := "─ " + l.text + " "
			pad := w - lipgloss.Width(label)
			if pad < 0 {
				pad = 0
			}
			out = append(out, []string{dimStyle.Render(label + strings.Repeat("─", pad))})
			continue
		}
		style := sayStyle
		if l.who == "you" {
			style = humanSayStyle
		}
		var group []string
		for _, ln := range wrapChat(l.who+": "+l.text, w) {
			group = append(group, style.Render(ln))
		}
		out = append(out, group)
	}
	if m.reactionPending && m.phase == phaseMatchOver {
		out = append(out, []string{dimStyle.Render(m.spin.View() + m.opp.DisplayName + " is typing…")})
	}
	return out
}

func (m Model) feedLines(w int) []string {
	var out []string
	for _, g := range m.feedLineGroups(w) {
		out = append(out, g...)
	}
	return out
}

// wrapChat greedy-wraps s to width w; continuation lines are indented 2.
// A single word longer than a line is hard-split so the panel edge holds.
func wrapChat(s string, w int) []string {
	if w < 8 {
		w = 8
	}
	var lines []string
	cur, pre := "", "" // pre: continuation indent once the first line is out
	add := func(word string) {
		for {
			sep := ""
			if cur != pre {
				sep = " "
			}
			if lipgloss.Width(cur+sep+word) <= w {
				cur += sep + word
				return
			}
			if cur != pre {
				lines = append(lines, cur)
				pre = "  "
				cur = pre
				continue
			}
			// word alone overflows the line: hard-split it
			r := []rune(word)
			cut := w - lipgloss.Width(cur)
			lines = append(lines, cur+string(r[:cut]))
			pre = "  "
			cur = pre
			word = string(r[cut:])
		}
	}
	for _, word := range strings.Fields(s) {
		add(word)
	}
	if cur != pre {
		lines = append(lines, cur)
	}
	return lines
}

// composeWithPanel joins the table column and the talk panel with a
// full-height separator bar; both columns are padded to equal height.
func (m Model) composeWithPanel(left string) string {
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	h := len(leftLines)
	body := make([]string, 0, h)
	body = append(body, dimStyle.Render("talk"))
	lines := m.feedLines(chatPanelWidth)
	if avail := h - 1; len(lines) > avail {
		lines = lines[len(lines)-avail:] // auto-scroll: newest at bottom
	}
	body = append(body, lines...)
	for len(body) < h {
		body = append(body, "")
	}
	sep := strings.TrimRight(strings.Repeat(" │ \n", h), "\n")
	panel := strings.Join(body, "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top,
		strings.Join(leftLines, "\n"), sep, panel) + "\n"
}

// renderBottomLog is the narrow/no-size fallback: the last few feed lines
// between the banner and the action bar.
func (m Model) renderBottomLog() string {
	w := m.width - 4
	if w < 20 {
		w = chatPanelWidth * 2
	}
	groups := m.feedLineGroups(w)
	// take whole messages from the end so the log never opens with an
	// orphan continuation line
	var lines []string
	for i := len(groups) - 1; i >= 0; i-- {
		if len(lines)+len(groups[i]) > chatLogLines && len(lines) > 0 {
			break
		}
		lines = append(groups[i], lines...)
	}
	if len(lines) > chatLogLines { // single message longer than the log
		lines = lines[len(lines)-chatLogLines:]
	}
	if len(lines) == 0 {
		return ""
	}
	return "  " + strings.Join(lines, "\n  ") + "\n"
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}
