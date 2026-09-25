package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wuhaohan772/showdown/internal/agent"
	"github.com/wuhaohan772/showdown/internal/debuglog"
	"github.com/wuhaohan772/showdown/internal/stats"
	"github.com/wuhaohan772/showdown/internal/tui"
)

func main() {
	agentFlag := flag.String("agent", "", "opponent agent key (claude, codex, gemini)")
	modelFlag := flag.String("model", "", "model for the agent CLI (claude: haiku/sonnet/opus; codex/gemini: passed through)")
	personalityFlag := flag.String("personality", "", "table persona: preset name (needler, unhinged, polite, silent, degen) or path to a .md file; default ~/.showdown/personality.md if present, else needler")
	stackFlag := flag.Int("stack", tui.DefaultStartStack, "starting chip stack per player")
	blindFlag := flag.Int("blind", tui.DefaultStartSB, "starting small blind (big blind is always 2x)")
	handsFlag := flag.Int("hands", tui.DefaultHandLimit, "cap the match at this many hands (0 = unlimited, play until someone busts; tied stacks at the cap continue until untied)")
	quiet := flag.Bool("quiet", false, "disable table talk")
	debug := flag.Bool("debug", false, "write a JSONL debug transcript to ~/.showdown/")
	noMouse := flag.Bool("no-mouse", false, "don't capture the mouse (clicking the mascot won't animate it; plain click-drag selects text again)")
	themeFlag := flag.String("theme", "", "color theme: auto (default), dark, or light; also read from SHOWDOWN_THEME")
	flag.Parse()
	tui.ApplyTheme(tui.ResolveTheme(tui.ThemeArg(*themeFlag)))

	var dlog *debuglog.Logger
	var dlogPath string
	if *debug || os.Getenv("SHOWDOWN_DEBUG") == "1" {
		dlogPath = debuglog.DefaultPath(time.Now())
		l, err := debuglog.New(dlogPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "debug log disabled: %v\n", err)
		} else {
			dlog = l
		}
	}

	st, err := stats.Load(stats.DefaultPath())
	if err != nil {
		st = stats.Stats{}
	}
	roster := agent.DetectRoster()

	var opp agent.Adapter
	var personaArg string
	useQuiet := *quiet
	startStack, startSB, handLimit := *stackFlag, *blindFlag, *handsFlag
	if *agentFlag != "" {
		if len(roster) == 0 {
			fmt.Fprintln(os.Stderr, "no agent CLIs found on PATH (looked for: claude, codex, gemini)")
			os.Exit(1)
		}
		for _, a := range roster {
			if a.Key == *agentFlag {
				opp = a
			}
		}
		if opp.Key == "" {
			fmt.Fprintf(os.Stderr, "agent %q not found (have: %v)\n", *agentFlag, keys(roster))
			os.Exit(1)
		}
		if *modelFlag != "" {
			opp.Model = *modelFlag
		}
		personaArg = *personalityFlag
	} else {
		opp, personaArg, useQuiet, startStack, startSB, handLimit, err = tui.RunMenu(roster, st, *modelFlag, *personalityFlag, *stackFlag, *blindFlag, *handsFlag, *quiet)
		if errors.Is(err, tui.ErrMenuQuit) {
			os.Exit(0)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	persona, truncated, err := agent.LoadPersonality(personaArg, agent.PersonalityPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if truncated {
		fmt.Fprintf(os.Stderr, "personality file truncated to %d chars\n", agent.PersonalityCap)
	}

	dir, _ := os.Getwd()
	m := tui.NewModel(opp, st, stats.DefaultPath(), useQuiet, dir, persona, startStack, startSB, handLimit, dlog)
	m = m.WithMouse(!*noMouse)
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if !*noMouse {
		opts = append(opts, tea.WithMouseCellMotion()) // clicks on the mascot
	}
	final, runErr := tea.NewProgram(m, opts...).Run()
	if fm, ok := final.(tui.Model); ok {
		fm.CloseSession() // release the persistent opponent process, if any
	}
	// Closing here means an agent call still in flight when user quits no-ops
	// its log write (write-after-close is a no-op); accepted tradeoff—never
	// block exit on the log.
	_ = dlog.Close()
	if dlog != nil {
		fmt.Fprintln(os.Stderr, "debug log:", dlogPath)
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}

func keys(roster []agent.Adapter) []string {
	out := make([]string, len(roster))
	for i, a := range roster {
		out[i] = a.Key
	}
	return out
}
