package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCustomAdapterViaEnv(t *testing.T) {
	t.Setenv("SHOWDOWN_AGENT_CMD", "testdata/stub.sh")
	roster := DetectRoster()
	if len(roster) != 1 || roster[0].Key != "custom" {
		t.Fatalf("roster = %+v, want single custom adapter", roster)
	}
	ask := roster[0].Asker(".", 10*time.Second)
	out, err := ask(context.Background(), "PROMPT")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if !strings.Contains(out, `"action": "call"`) {
		t.Errorf("out = %q", out)
	}
}

func TestAskerTimeout(t *testing.T) {
	t.Setenv("SHOWDOWN_AGENT_CMD", "sleep")
	roster := DetectRoster()
	ask := roster[0].Asker(".", 200*time.Millisecond)
	start := time.Now()
	_, err := ask(context.Background(), "5") // sleep 5 — must be killed
	if err == nil {
		t.Fatal("want timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("timeout did not kill the process promptly")
	}
}

func TestKnownAdapterArgs(t *testing.T) {
	for _, a := range knownAdapters {
		args := a.Args("PROMPT")
		found := false
		for _, x := range args {
			if x == "PROMPT" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: prompt not in args %v", a.Key, args)
		}
	}
}

func TestWhitespaceCustomCmdFallsBack(t *testing.T) {
	t.Setenv("SHOWDOWN_AGENT_CMD", "   ")
	roster := DetectRoster() // must not panic
	for _, a := range roster {
		if a.Key == "custom" {
			t.Errorf("whitespace-only SHOWDOWN_AGENT_CMD produced custom adapter")
		}
	}
}
