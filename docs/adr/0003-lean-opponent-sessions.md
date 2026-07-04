# ADR-0003: Lean opponent sessions

## Status

Accepted (2026-07-04)

## Context

Each opponent decision spawns a full `claude -p` session: ~20k context
tokens, of which ~11k are tool schemas (Bash, Read, Edit, …) and ~8k the
default Claude Code system prompt. The opponent never calls a tool — it
answers with one JSON object — so both are dead weight billed on every
decision and every reaction. Measured cold cost: $0.0215/decision (haiku).

The human explicitly prioritized token compression over latency: any
sub-5-second response is acceptable and equivalent.

## Decision

The claude adapter passes `--disallowedTools '*'` (drops tool schemas)
and a static minimal `--system-prompt` with hard brevity rules (replaces
the default system prompt). Measured result: ~2k context tokens,
$0.0039/decision — a 90% token cut.

The system prompt must remain a static string so the cached prefix is
identical across calls (prompt cache warms after the first call in a
5-minute window).

## Consequences

- The opponent loses the Claude Code persona from its default system
  prompt; the game prompt (`prompt_template.md`) carries the identity
  ("you are {agent_name}…") instead.
- CLAUDE.md / memory files still load as dynamic system-prompt sections
  appended after a custom `--system-prompt` (ADR-0002 intact).
- The opponent can no longer run tools mid-decision — also closes the
  door on accidental file reads/writes during a poker hand.
- codex/gemini adapters are unchanged; no equivalent flags measured yet.
