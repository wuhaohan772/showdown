package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/debuglog"
	"github.com/haohanwu/showdown/internal/stats"
	"github.com/haohanwu/showdown/internal/tui"
)

func main() {
	agentFlag := flag.String("agent", "", "opponent agent key (claude, codex, gemini)")
	quiet := flag.Bool("quiet", false, "disable table talk")
	debug := flag.Bool("debug", false, "write a JSONL debug transcript to ~/.showdown/")
	flag.Parse()

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
	} else {
		opp, err = tui.RunPicker(roster, st)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	dir, _ := os.Getwd()
	m := tui.NewModel(opp, st, stats.DefaultPath(), *quiet, dir, dlog)
	_, runErr := tea.NewProgram(m, tea.WithAltScreen()).Run()
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
