# Shove-tendency tracker design

## Problem

The agent's poker decisions come entirely from an LLM prompt each turn (`internal/agent/prompt_template.md` + `RequestData`), not a numeric strategy engine. The per-hand `hand_state` block (`BuildHandState` in prompt.go) gives full action detail for the *current* hand, but once a hand ends, `agentHandSummary` (internal/tui/app.go:307) collapses it to just `"Hand N: <winner> won <pot> (<how>)"` — it drops whether the hand ended via an all-in shove or a small bet.

Result: the match digest fed back to the agent (`Digest.String()`) carries no persistent signal about opponent aggression across hands. A human player who gets shoved on 4 times in a row starts reading it as a bluff-heavy pattern and calls lighter; the agent currently re-evaluates each shove in isolation and tends to fold to repeated all-ins as if each were independently strong.

## Goal

Give the agent a persistent, low-noise read on how often the opponent shoves all-in and how often the agent has folded to it, so it can start exploiting a bluff-heavy shover instead of folding to the same move every time. Scope is intentionally narrow — shove frequency only, not a full opponent-tendency profile (VPIP/aggression/fold-to-raise/etc.). Shove frequency is the single highest-signal, easiest-to-detect heads-up tell, and a broad multi-stat profile risks tiny-sample noise dominating early hands in the short matches this game typically runs. Broader opponent modeling is deferred; this spec establishes the counter → digest line → prompt-weight pattern a future broader system would reuse.

## Design

### 1. Detection: `agent.DetectShove`

New pure function in `internal/agent` (package `agent`), no TUI dependency:

```go
func DetectShove(log []poker.LogItem, agentSeat int, r *poker.Result) (sawShove, foldedToShove bool)
```

- `sawShove` = true if any `LogItem` in `log` has `Seat != agentSeat && AllIn == true`.
- `foldedToShove` = `sawShove && !r.Showdown && !r.Split && r.Winner != agentSeat` (hand ended by a fold, and the agent was the one who folded).

Pure and independently testable against `poker.LogItem`/`poker.Result` fixtures — no hand/match/session setup needed.

### 2. Counters: `Digest`

`internal/agent/digest.go` gains three fields and one method:

```go
type Digest struct {
    events     []string
    handTalk   []string
    startStack int
    handsPlayed    int
    opponentShoves int
    foldedToShove  int
}

func (d *Digest) RecordHand(sawShove, foldedToShove bool) {
    d.handsPlayed++
    if sawShove {
        d.opponentShoves++
    }
    if foldedToShove {
        d.foldedToShove++
    }
}
```

### 3. Surfacing: `Digest.String()`

Once `handsPlayed >= 3`, `String()` prepends one line before the existing event log:

```
Opponent tendency: shoved all-in {opponentShoves} of {handsPlayed} hands; you folded to {foldedToShove} of those shoves.
```

Below the threshold (`handsPlayed < 3`), the line is omitted entirely — no "not enough data yet" filler, to keep the digest token-lean (matches the project's existing token-compression-first cost posture, see `ADR-0003`/[[showdown-feature-roadmap]]).

If `opponentShoves == 0`, the line is also omitted (nothing to report).

### 4. Prompt change

One new bullet appended as the **last** item in `prompt_template.md`'s "Format rules" section (after the existing needling-doctrine bullet), instructing the model to use the tendency line when present: a high shove rate paired with folds on your side is a bluff tell — widen your calling range against future shoves; a low or absent shove rate means respect a shove as strength. Last position matters: prior work on this template found behavioral rules placed in the intro/body paragraphs get ignored live, while the last Format-rules bullet reliably lands (recency wins) — see [[showdown-feature-roadmap]] on the needling-doctrine fix. Framed as reasoning guidance, consistent with the template's existing "you play real poker... think about betting history before you act" line — not a hard override of hand-reading.

### 5. Wiring

`internal/tui/app.go`, in the hand-finish handler (~line 286), immediately before the existing `m.digest.EndHand(summary)` call:

```go
sawShove, foldedToShove := agent.DetectShove(m.hand.Log, m.agentSeat(), r)
m.digest.RecordHand(sawShove, foldedToShove)
```

Must run before `m.hand` is reset/replaced for the next hand (same constraint the existing summary-building code already respects).

### Data flow recap

`finishHand` (per-hand, has `m.hand.Log` + `r *poker.Result`) → `DetectShove` (pure) → `Digest.RecordHand` (counters) → `Digest.String()` (rendered into `RequestData.MatchDigest`, both the priming-turn prompt and the end-of-match reaction prompt already read this same string — no new plumbing needed there).

## Testing

- `internal/agent/digest_test.go` (new file): counter math via `RecordHand`; `String()` omits the tendency line below 3 hands and when `opponentShoves == 0`; correct wording/numbers at and above threshold.
- `internal/agent/shove_test.go` (new, or add to an existing agent test file): `DetectShove` cases — opponent shoves and agent folds; opponent shoves and agent calls/wins at showdown; no shove in the hand; split pot; agent itself shoves (must not count as opponent shove).
- No automated test for "the model's decisions actually change" — LLM behavior isn't unit-testable. Verify manually the same way say-behavior changes have been verified before (per [[model-talk-quality-and-cache]]): a live multi-hand playtest, `--debug` transcript, scripted opponent shoving 3-4x in a row, eyeball whether the agent's fold rate to later shoves visibly loosens and whether its "say" lines start referencing the pattern. Run against both haiku and sonnet per the existing canary protocol, since this is a prompt-template change.

## Out of scope

- Broader opponent-tendency stats (VPIP, fold-to-raise, bluff/value ratio at showdown, positional tendencies) — deferred; revisit if the narrow tracker proves out.
- Any change to the numeric poker engine (`internal/poker`) — decisions remain fully LLM-driven; this only changes what information reaches the prompt.
