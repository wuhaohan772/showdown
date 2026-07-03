# Agent personality comes from native memory loading, not injection

We want trash talk to be personal — the opponent is the agent you actually work with, and it should know you. We decided the game never reads or injects the agent's memory files (CLAUDE.md, AGENTS.md, memory dirs); instead each adapter invokes the CLI so that it loads its own context naturally, exactly as in a normal working session. Harvesting files ourselves would work uniformly across CLIs but means rummaging in another tool's private state with brittle paths.

## Consequences

- The opponent's knowledge of you depends on where showdown is launched (project dir → project-flavored trash talk).
- The agent may surface personal memory content on screen; acceptable since everything stays on the user's own machine.
- "Instinct" difficulty disables tool *execution* only — native context loading must stay on, or the personality dies.
