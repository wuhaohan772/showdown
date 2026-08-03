package agent

import (
	"testing"

	"github.com/wuhaohan772/showdown/internal/poker"
)

func TestDetectShove(t *testing.T) {
	cases := []struct {
		name           string
		log            []poker.LogItem
		agentSeat      int
		result         *poker.Result
		wantSaw        bool
		wantFoldedToIt bool
	}{
		{
			name: "opponent shoves, agent folds",
			log: []poker.LogItem{
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Raise, To: 3000}, AllIn: true},
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Fold}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: 1, Pot: 3000},
			wantSaw:        true,
			wantFoldedToIt: true,
		},
		{
			name: "opponent shoves, agent calls and wins at showdown",
			log: []poker.LogItem{
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Raise, To: 3000}, AllIn: true},
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Call}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: 0, Pot: 6000, Showdown: true, Desc: "pair of kings"},
			wantSaw:        true,
			wantFoldedToIt: false,
		},
		{
			name: "no shove in the hand",
			log: []poker.LogItem{
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Call}},
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Check}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: 1, Pot: 200, Showdown: true, Desc: "ace high"},
			wantSaw:        false,
			wantFoldedToIt: false,
		},
		{
			name: "split pot after opponent shove",
			log: []poker.LogItem{
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Raise, To: 3000}, AllIn: true},
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Call}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: -1, Split: true, Pot: 6000, Showdown: true, Desc: "chop"},
			wantSaw:        true,
			wantFoldedToIt: false,
		},
		{
			name: "agent itself shoves — must not count as opponent shove",
			log: []poker.LogItem{
				{Seat: 0, Street: poker.Preflop, Act: poker.Action{Type: poker.Raise, To: 3000}, AllIn: true},
				{Seat: 1, Street: poker.Preflop, Act: poker.Action{Type: poker.Fold}},
			},
			agentSeat:      0,
			result:         &poker.Result{Winner: 0, Pot: 3000},
			wantSaw:        false,
			wantFoldedToIt: false,
		},
	}
	for _, c := range cases {
		saw, folded := DetectShove(c.log, c.agentSeat, c.result)
		if saw != c.wantSaw || folded != c.wantFoldedToIt {
			t.Errorf("%s: DetectShove() = (%v, %v), want (%v, %v)", c.name, saw, folded, c.wantSaw, c.wantFoldedToIt)
		}
	}
}
