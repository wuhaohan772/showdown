package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Session owns one long-lived agent CLI process speaking the stream-json
// protocol (spec: docs/superpowers/specs/2026-07-04-persistent-session-design.md).
// The conversation is the opponent's match memory: turn 1 carries the full
// prompt, later turns small deltas, so the API prompt cache stays warm
// (~$0.001/decision vs ~$0.015 stateless) and process spawn is paid once.
// The TUI's one-turn-at-a-time flow means Ask is never called concurrently.
type Session struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	results chan streamResult
	timeout time.Duration

	mu       sync.Mutex
	dead     bool
	primed   bool
	lastCost float64
}

type streamResult struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

type streamUserMsg struct {
	Type    string `json:"type"`
	Message struct {
		Role    string          `json:"role"`
		Content []streamContent `json:"content"`
	} `json:"message"`
}

type streamContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// StartSession launches the CLI and its reader goroutine. timeout bounds
// each turn (the TUI passes its decisionTimeout).
func StartSession(bin string, args, env []string, dir string, timeout time.Duration) (*Session, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	s := &Session{cmd: cmd, stdin: stdin, results: make(chan streamResult, 1), timeout: timeout}
	go s.readLoop(stdout)
	return s, nil
}

// readLoop forwards result events; other event types (system, assistant,
// rate_limit_event) are skipped. Channel close = stdout EOF = process gone.
func (s *Session) readLoop(stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var r streamResult
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			continue
		}
		if r.Type == "result" {
			s.results <- r
		}
	}
	close(s.results)
}

// Ask sends one user message and blocks for its result event. Any failure
// (write error, EOF, timeout, malformed stream) kills the session — the
// caller's fallback path takes over and a later decision may start a fresh
// session.
func (s *Session) Ask(ctx context.Context, prompt string) (Response, error) {
	if !s.Alive() {
		return Response{}, fmt.Errorf("session dead")
	}
	var msg streamUserMsg
	msg.Type = "user"
	msg.Message.Role = "user"
	msg.Message.Content = []streamContent{{Type: "text", Text: prompt}}
	b, err := json.Marshal(msg)
	if err != nil {
		return Response{}, err
	}
	if _, err := s.stdin.Write(append(b, '\n')); err != nil {
		s.kill()
		return Response{}, fmt.Errorf("session write: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	select {
	case r, ok := <-s.results:
		if !ok {
			s.kill()
			return Response{}, fmt.Errorf("session closed stream")
		}
		// Error result (e.g. rate limit, max turns): treat as fatal — the TUI's
		// stateless fallback path will take over for this decision, and the
		// session is dead for future decisions.
		if r.IsError || (r.Subtype != "" && r.Subtype != "success") {
			s.kill()
			return Response{}, fmt.Errorf("session error result: subtype=%q is_error=%v", r.Subtype, r.IsError)
		}
		s.mu.Lock()
		cost := r.TotalCostUSD - s.lastCost
		s.lastCost = r.TotalCostUSD
		s.mu.Unlock()
		return Response{Text: r.Result, Usage: &Usage{
			InputTokens:  r.Usage.InputTokens,
			OutputTokens: r.Usage.OutputTokens,
			CacheRead:    r.Usage.CacheReadInputTokens,
			CacheWrite:   r.Usage.CacheCreationInputTokens,
			CostUSD:      cost,
		}}, nil
	case <-tctx.Done():
		s.kill()
		return Response{}, fmt.Errorf("session turn: %w", tctx.Err())
	}
}

// Asker adapts the session to the plain Asker function type so
// debuglog.WrapAsker and GetDecision compose unchanged.
func (s *Session) Asker() Asker { return s.Ask }

func (s *Session) Alive() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.dead
}

func (s *Session) Primed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.primed
}

func (s *Session) MarkPrimed() {
	s.mu.Lock()
	s.primed = true
	s.mu.Unlock()
}

func (s *Session) kill() {
	s.mu.Lock()
	if s.dead {
		s.mu.Unlock()
		return
	}
	s.dead = true
	s.mu.Unlock()
	_ = s.stdin.Close()
	_ = s.cmd.Process.Kill()
	go func() { _ = s.cmd.Wait() }() // reap; never block a decision on it
}

// Close ends the session gracefully: stdin EOF lets the CLI exit on its
// own, with a kill safety net. Nil-safe.
func (s *Session) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.dead {
		s.mu.Unlock()
		return
	}
	s.dead = true
	s.mu.Unlock()
	_ = s.stdin.Close()
	done := make(chan struct{})
	go func() { _ = s.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
	}
}
