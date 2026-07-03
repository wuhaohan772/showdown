package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
	"github.com/haohanwu/showdown/internal/tui"
)

func main() {
	agentFlag := flag.String("agent", "", "opponent agent key (claude, codex, gemini)")
	quiet := flag.Bool("quiet", false, "disable table talk")
	flag.Parse()

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
	m := tui.NewModel(opp, st, stats.DefaultPath(), *quiet, dir)
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
