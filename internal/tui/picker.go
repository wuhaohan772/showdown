package tui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

// RunPicker chooses the opponent before the TUI starts.
func RunPicker(roster []agent.Adapter, st stats.Stats) (agent.Adapter, error) {
	if len(roster) == 0 {
		return agent.Adapter{}, fmt.Errorf("no agent CLIs found on PATH (looked for: claude, codex, gemini)")
	}
	if len(roster) == 1 {
		return roster[0], nil
	}
	fmt.Println("♠ choose your opponent:")
	for i, a := range roster {
		fmt.Printf("  %d. %-12s %s\n", i+1, a.DisplayName, st.Line(a.Key))
	}
	fmt.Print("> ")
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return agent.Adapter{}, fmt.Errorf("no selection")
	}
	n, err := strconv.Atoi(strings.TrimSpace(sc.Text()))
	if err != nil || n < 1 || n > len(roster) {
		return agent.Adapter{}, fmt.Errorf("pick a number 1-%d", len(roster))
	}
	return roster[n-1], nil
}
