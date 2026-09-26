package tui

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/wuhaohan772/showdown/internal/agent"
	"github.com/wuhaohan772/showdown/internal/poker"
	"github.com/wuhaohan772/showdown/internal/stats"
)

// TestRecordDemo writes docs/demo.cast by driving the real Model through one
// scripted hand — no agent CLI, no API spend, same frames the game draws.
// It is skipped unless SHOWDOWN_RECORD_DEMO=1; see scripts/record-demo.sh.
//
// The script is fixed but the deck is not, so the driver reacts to the hand
// it is dealt: scripted intents fall back to a legal action when the hand
// does not allow the one the script asked for.
func TestRecordDemo(t *testing.T) {
	if os.Getenv("SHOWDOWN_RECORD_DEMO") != "1" {
		t.Skip("set SHOWDOWN_RECORD_DEMO=1 to re-record docs/demo.cast")
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
	ApplyTheme("dark")

	const (
		cols, rows = 100, 24
	)
	// seed 23 deals clawd trip sevens, so the demo ends on its win pose.
	// SHOWDOWN_DEMO_SEED overrides it when picking a new deal.
	seed := int64(23)
	if v := os.Getenv("SHOWDOWN_DEMO_SEED"); v != "" {
		fmt.Sscan(v, &seed)
	}
	ad := agent.Adapter{Key: "claude", DisplayName: "Claude Code", Model: "sonnet", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", false, ".", "standard", DefaultStartStack, DefaultStartSB, 0, nil)
	m.rng = rand.New(rand.NewSource(seed))
	m.spin.Spinner.FPS = 0 // the recorder drives frames, not wall-clock

	rec := &castRecorder{cols: cols, rows: rows}
	step := func(msg tea.Msg) {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	step(tea.WindowSizeMsg{Width: cols, Height: rows})

	// hold renders n frames of whatever is on screen, ticking animations.
	hold := func(n int) {
		for i := 0; i < n; i++ {
			step(animTickMsg{})
			if m.phase == phaseRunout {
				step(runoutTickMsg{})
			}
			rec.frame(m.View())
		}
	}

	taunts := []string{
		"Read your CLAUDE.md before we started. Ship date's slipping and you're in here playing cards.",
		"Raise it. Every chip you lose is a commit you don't get to push tonight.",
		"Call. I'm renaming your main branch if this holds up.",
	}
	// readFor holds until the typewriter finishes a line, then long enough
	// to read it: ~3 words a second, 2.5s minimum. Without this the demo
	// moved on before the talk had finished typing.
	readFor := func(say string) {
		for i := 0; m.typeIdx >= 0 && i < 400; i++ {
			hold(1)
		}
		n := len(strings.Fields(say)) * 7
		if n < 50 {
			n = 50
		}
		hold(n)
	}

	nextTaunt := 0
	agentSays := func() string {
		if nextTaunt >= len(taunts) {
			return ""
		}
		s := taunts[nextTaunt]
		nextTaunt++
		return s
	}

	step(startHandMsg{})
	hold(40) // take in the table and the header

	// humanScript is consumed in order; each entry is a preferred action for
	// one human turn, and falls back to check/call when it is not legal.
	humanScript := []string{"raise"} // then check/call down to the showdown
	humanTurn := 0
	playHuman := func() {
		want := "call"
		if humanTurn < len(humanScript) {
			want = humanScript[humanTurn]
		}
		humanTurn++
		legal := legalSet(m.hand.LegalActions())
		switch {
		case want == "raise" && legal[poker.Raise]:
			step(key("r"))
			hold(6)
			for _, r := range fmt.Sprint(m.hand.MinRaiseTo() * 2) {
				step(key(string(r)))
				hold(3)
			}
			step(tea.KeyMsg{Type: tea.KeyEnter})
		case want == "allin" && legal[poker.Raise]:
			step(key("a"))
		case want == "fold" && legal[poker.Fold]:
			step(key("f"))
		case legal[poker.Check]:
			step(key("c"))
		default:
			step(key("c"))
		}
		hold(16)
	}

	// playAgent answers the agent's turn with a canned decision: raise when
	// the script still has a taunt for it, otherwise check or call.
	playAgent := func() {
		legal := legalSet(m.hand.LegalActions())
		act := poker.Action{Type: poker.Call}
		switch {
		case legal[poker.Check]:
			act = poker.Action{Type: poker.Check}
		case !legal[poker.Call]:
			act = poker.Action{Type: poker.Fold}
		}
		hold(16) // let clawd's eyes move while it "thinks"
		say := agentSays()
		step(decisionMsg{act: act, say: say})
		if say != "" {
			readFor(say)
		} else {
			hold(16)
		}
	}

	for range [40]struct{}{} {
		switch m.phase {
		case phaseHumanTurn:
			playHuman()
		case phaseAgentTurn:
			playAgent()
		case phaseRunout:
			hold(6)
		case phaseDealing:
			hold(4)
		}
		if m.phase == phaseHandEnd {
			break
		}
	}
	hold(80) // rest on the showdown and the result
	if r := m.hand.Result(); r != nil {
		t.Logf("DEMO seed=%d winner_is_agent=%v showdown=%v desc=%q banner=%q",
			seed, r.Winner == m.agentSeat(), r.Showdown, r.Desc, m.banner)
	}

	if err := rec.write("../../docs/demo.cast"); err != nil {
		t.Fatalf("write cast: %v", err)
	}
	t.Logf("wrote docs/demo.cast: %d frames, %.1fs", rec.frames, rec.clock)
}

func legalSet(acts []poker.ActionType) map[poker.ActionType]bool {
	out := map[poker.ActionType]bool{}
	for _, a := range acts {
		out[a] = true
	}
	return out
}

// castRecorder collects screen frames as an asciicast v2 file. Frames are
// spaced one animation frame apart, which is what the game itself uses.
type castRecorder struct {
	cols, rows int
	events     []string
	clock      float64
	frames     int
	last       string
}

func (r *castRecorder) frame(view string) {
	r.clock += animFrame.Seconds()
	if view == r.last {
		return // nothing moved: let the previous frame hold
	}
	r.last = view
	r.frames++
	payload, _ := json.Marshal("\x1b[H\x1b[2J" + strings.ReplaceAll(view, "\n", "\r\n"))
	r.events = append(r.events, fmt.Sprintf("[%.3f, \"o\", %s]", r.clock, payload))
}

func (r *castRecorder) write(path string) error {
	header := fmt.Sprintf(`{"version": 2, "width": %d, "height": %d, "title": "showdown", "env": {"TERM": "xterm-256color"}}`,
		r.cols, r.rows)
	body := header + "\n" + strings.Join(r.events, "\n") + "\n"
	return os.WriteFile(path, []byte(body), 0o644)
}
