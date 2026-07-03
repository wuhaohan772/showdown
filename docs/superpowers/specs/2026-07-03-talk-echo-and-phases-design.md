# Trash-Talk Echo & Phase Widening — Design

Date: 2026-07-03
Status: approved

## Purpose

Player trash talk (`t`) works end-to-end (verified: typed lines reach the
agent's `=== TABLE TALK THIS HAND ===` prompt section), but from the
player's seat the feature looks absent:

1. The typed line is never rendered — it vanishes on enter.
2. `t` only works during the player's betting turn. It is dead while the
   agent is thinking and at hand end — exactly when the agent's reaction
   lands and the player wants to fire back.

This feature adds an on-screen echo of the player's last line and widens
when `t` is accepted. No engine changes, no new agent CLI calls.

## Scope

All changes in `internal/tui/app.go` (+ its tests). The digest, prompt,
and engine layers are untouched.

## Echo

- New `Model` field `humanSay string`.
- Set in `confirmInput` on the talk path (`m.humanSay = val` when
  non-empty), cleared in `startHand` alongside `agentSay`.
- `View` renders it directly under the `♥ YOU   stack: N` row, one line:
  `you: "<line>"`, using `dimStyle` (italic yellow `sayStyle` stays
  reserved for the agent).
- Hidden when `--quiet` is set, matching how `agentSay` is suppressed.

## Phase widening

New `Model` field `talkReturn phase`: the phase to restore when talk
input closes. Set whenever talk input opens; restored on both enter
(`confirmInput`) and esc. This replaces the current hardcoded return to
`phaseHumanTurn`, which becomes wrong once talk can open from other
phases.

`t` opens talk input from:

- `phaseHumanTurn` (unchanged behavior, now via `talkReturn`).
- `phaseHandEnd` — after typing, phase returns to `phaseHandEnd`; the
  line is buffered in the digest's hand talk and reaches the agent in
  the next hand's TALK section (the hand-end reaction request has
  already been sent by then; a reply loop was considered and declined).
- `phaseAgentTurn` — the line reaches the agent at its next decision
  prompt (same hand if there is one, else via the digest archive).

Excluded: `phaseRunout` (talk input would freeze the board-reveal
animation, whose ticks only advance in `phaseRunout`), `phaseMatchOver`,
and the raise-input phase.

### Race: decision arrives while typing

If the agent's `decisionMsg` lands while the player is in talk input
(opened from `phaseAgentTurn`), the decision applies and the game
advances normally, but the input must survive: `Update`'s `decisionMsg`
case notes `wasTyping := m.phase == phaseTalkInput` before applying;
after `advance()` (or `finishHand`) sets the new phase, if `wasTyping`
the model stores that new phase in `talkReturn` and restores
`phaseTalkInput`. The player finishes their sentence; enter drops them
into the post-decision phase.

Sub-case: the decision triggers a showdown, so the post-decision phase
is `phaseRunout`. Runout ticks normally only advance the reveal while
`m.phase == phaseRunout`; with talk input preserved they must keep
working. The `runoutTickMsg` handler therefore also advances when
`m.phase == phaseTalkInput && m.talkReturn == phaseRunout`, updating
`talkReturn` (not `m.phase`) to `phaseHandEnd` when the reveal
completes. The board animates behind the input box and enter lands the
player in whatever phase the runout reached.

The raise-input phase cannot coincide with `decisionMsg` (raise input
only opens on the player's turn), so it needs no equivalent handling.

## Quiet mode

Unchanged semantics: `t` still accepted, lines still enter the digest
(they already do today), but the echo — like the agent's say line — is
not rendered.

## Error handling

None new: empty talk input remains a no-op (no echo, no digest entry);
esc cancels and restores the origin phase.

## Testing

`internal/tui/app_test.go`:

- Echo: submit talk on player's turn → `View()` contains `you: "…"`;
  cleared after next `startHand`; absent in quiet mode.
- Hand end: fold → `phaseHandEnd` → `t` opens input, enter returns to
  `phaseHandEnd`, line present in digest hand talk.
- Agent turn + race: enter talk input during `phaseAgentTurn`, deliver
  `decisionMsg` mid-typing → still `phaseTalkInput`; enter → phase equals
  the post-decision phase; the applied action took effect.
- Runout sub-case: mid-typing decision that causes a showdown →
  `runoutTickMsg`s still advance the reveal while typing, and
  `talkReturn` becomes `phaseHandEnd` when the board is fully revealed.
- Esc: from hand-end talk input, esc returns to `phaseHandEnd`.
