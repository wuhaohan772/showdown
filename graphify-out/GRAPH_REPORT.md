# Graph Report - /Users/haohanwu/code/showdown  (2026-07-03)

## Corpus Check
- 4 files · ~35,065 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 320 nodes · 653 edges · 21 communities (14 shown, 7 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 75 edges (avg confidence: 0.8)
- Token cost: 25,479 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_TUI App Core (Model & View)|TUI App Core (Model & View)]]
- [[_COMMUNITY_Cards & Hand Evaluation|Cards & Hand Evaluation]]
- [[_COMMUNITY_Debug Logger (debuglog)|Debug Logger (debuglog)]]
- [[_COMMUNITY_Poker Hand Tests|Poker Hand Tests]]
- [[_COMMUNITY_TUI App Tests|TUI App Tests]]
- [[_COMMUNITY_Stats & Agent Roster|Stats & Agent Roster]]
- [[_COMMUNITY_Agent Decision Parsing|Agent Decision Parsing]]
- [[_COMMUNITY_Poker Hand Engine|Poker Hand Engine]]
- [[_COMMUNITY_Debug & Talk Feature Specs|Debug & Talk Feature Specs]]
- [[_COMMUNITY_Agent Protocol Docs & ADRs|Agent Protocol Docs & ADRs]]
- [[_COMMUNITY_Match & Blinds|Match & Blinds]]
- [[_COMMUNITY_Talk Digest|Talk Digest]]
- [[_COMMUNITY_Key Hint Feature Docs|Key Hint Feature Docs]]
- [[_COMMUNITY_Prompt Spike Script|Prompt Spike Script]]
- [[_COMMUNITY_Stub Shell Script|Stub Shell Script]]
- [[_COMMUNITY_Stub Agent Script|Stub Agent Script]]
- [[_COMMUNITY_Roster Type|Roster Type]]
- [[_COMMUNITY_Adapter Type|Adapter Type]]
- [[_COMMUNITY_Rand Helper|Rand Helper]]
- [[_COMMUNITY_Test Helper|Test Helper]]
- [[_COMMUNITY_Module Root|Module Root]]

## God Nodes (most connected - your core abstractions)
1. `Model` - 27 edges
2. `Hand` - 25 edges
3. `NewHand()` - 18 edges
4. `testModel()` - 18 edges
5. `key()` - 18 edges
6. `Card` - 15 edges
7. `GetDecision()` - 13 edges
8. `mustApply()` - 13 edges
9. `newTestHand()` - 12 edges
10. `NewModel()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewModel()`  [INFERRED]
  main.go → internal/tui/app.go
- `main()` --calls--> `DefaultPath()`  [INFERRED]
  main.go → internal/debuglog/debuglog.go
- `main()` --calls--> `New()`  [INFERRED]
  main.go → internal/debuglog/debuglog.go
- `Showdown` --references--> `Debug Transcript Log Design`  [EXTRACTED]
  README.md → docs/superpowers/specs/2026-07-03-debug-transcript-log-design.md
- `TestRenderPromptFillsEverything()` --calls--> `RenderPrompt()`  [INFERRED]
  internal/agent/prompt_test.go → internal/agent/prompt.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Key Hint Row View Phase Switch Implementation** — docs_superpowers_specs_2026_07_03_key_hints_design_key_hint_row, docs_superpowers_specs_2026_07_03_key_hints_design_phase_agent_turn_hint, docs_superpowers_specs_2026_07_03_key_hints_design_phase_hand_end_hint, internal_tui_app_view [EXTRACTED 1.00]
- **Debug Event Capture Pipeline** — docs_superpowers_specs_2026_07_03_debug_transcript_log_design_logger, docs_superpowers_specs_2026_07_03_debug_transcript_log_design_wrapasker, docs_superpowers_specs_2026_07_03_debug_transcript_log_design_apply_helper [EXTRACTED 1.00]
- **Phase-Aware Talk Input Management** — docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_talkreturn_field, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_opentalk_helper, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_phase_widening [EXTRACTED 1.00]
- **Async State Preservation Pattern** — docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_talkreturn_field, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_decision_race_handling, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_runout_tick_handling [EXTRACTED 1.00]
- **Decision Pipeline: Request → Decision → Action** — context_md_decision_request, context_md_decision, context_md_fallback_action [INFERRED 0.85]
- **ADR Implementation in Code** — docs_adr_0001_stateless_decision_requests, docs_adr_0002_native_memory_loading, superpowers_sdd_task_9_report_get_decision, superpowers_sdd_task_10_brief_asker_method [INFERRED 0.75]

## Communities (21 total, 7 thin omitted)

### Community 0 - "TUI App Core (Model & View)"
Cohesion: 0.10
Nodes (26): Action, ActionType, Adapter, Card, Cmd, Digest, Hand, cardStrings() (+18 more)

### Community 1 - "Cards & Hand Evaluation"
Cohesion: 0.12
Nodes (30): Rand, NewDeck(), ParseCard(), T, TestCardString(), TestDeckDeals52Unique(), TestDeckShuffled(), TestParseCardRoundTrip() (+22 more)

### Community 2 - "Debug Logger (debuglog)"
Cohesion: 0.12
Nodes (22): Asker, Logger, File, T, TestWrapAskerLogsError(), TestWrapAskerNilLogger(), TestWrapAskerPassthroughAndLog(), WrapAsker() (+14 more)

### Community 3 - "Poker Hand Tests"
Cohesion: 0.18
Nodes (27): BuildHandState(), T, TestBuildHandState(), TestBuildHandStatePerspectiveIndependentOfActor(), TestRenderPromptFillsEverything(), Rand, NewHand(), T (+19 more)

### Community 4 - "TUI App Tests"
Cohesion: 0.24
Nodes (28): NewModel(), KeyMsg, key(), readLogEvents(), TestAgentDecisionLogged(), TestAgentDecisionMsgApplies(), TestDebugLogCapturesHandFlow(), TestFallbackNoticeVisibleInQuietMode() (+20 more)

### Community 5 - "Stats & Agent Roster"
Cohesion: 0.11
Nodes (16): Adapter, Duration, DetectRoster(), T, TestAskerTimeout(), TestCustomAdapterViaEnv(), TestKnownAdapterArgs(), TestWhitespaceCustomCmdFallsBack() (+8 more)

### Community 6 - "Agent Decision Parsing"
Cohesion: 0.19
Nodes (22): Asker, Decision, RequestData, Context, ExtractDecision(), FallbackAction(), GetDecision(), GetReaction() (+14 more)

### Community 7 - "Poker Hand Engine"
Cohesion: 0.19
Nodes (5): Hand, LogItem, Result, SeatState, Street

### Community 8 - "Debug & Talk Feature Specs"
Cohesion: 0.16
Nodes (18): Debug Transcript Log Implementation Plan, Trash-Talk Echo & Phase Widening Implementation Plan, apply Helper, Bug Report Protocol, Debug Transcript Log Design, internal/debuglog Package, JSONL Format, Logger (+10 more)

### Community 9 - "Agent Protocol Docs & ADRs"
Cohesion: 0.15
Nodes (17): Agent Adapter, Career Record, Decision, Decision Request, Fallback Action, Game, Hand-End Reaction, Match (+9 more)

### Community 10 - "Match & Blinds"
Cohesion: 0.22
Nodes (6): NewMatch(), T, TestBlindSchedule(), TestButtonAlternates(), TestNextHandAndBust(), Match

### Community 11 - "Talk Digest"
Cohesion: 0.25
Nodes (3): Digest, NewDigest(), TestDigestFlow()

### Community 12 - "Key Hint Feature Docs"
Cohesion: 0.29
Nodes (7): Key Hint Rows Implementation Plan, TestKeyHintRows, Banner Cleanup, Discoverability Gap, Key Hint Row, Phase Agent Turn Hint Case, Phase Hand End Hint Case

### Community 13 - "Prompt Spike Script"
Cohesion: 0.53
Nodes (5): build_prompt(), extract_json(), main(), Find the last parseable JSON object with an 'action' key., run_one()

## Knowledge Gaps
- **20 isolated node(s):** `github.com/haohanwu/showdown`, `stub.sh script`, `Record`, `stub-agent.sh script`, `Game` (+15 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **7 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Hand` connect `Poker Hand Engine` to `Cards & Hand Evaluation`, `Poker Hand Tests`?**
  _High betweenness centrality (0.084) - this node is a cross-community bridge._
- **Why does `Asker` connect `Agent Decision Parsing` to `Stats & Agent Roster`?**
  _High betweenness centrality (0.061) - this node is a cross-community bridge._
- **Are the 12 inferred relationships involving `NewHand()` (e.g. with `TestBuildHandState()` and `TestBuildHandStatePerspectiveIndependentOfActor()`) actually correct?**
  _`NewHand()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/haohanwu/showdown`, `stub.sh script`, `Record` to the rest of the system?**
  _23 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `TUI App Core (Model & View)` be split into smaller, more focused modules?**
  _Cohesion score 0.10452961672473868 - nodes in this community are weakly interconnected._
- **Should `Cards & Hand Evaluation` be split into smaller, more focused modules?**
  _Cohesion score 0.11522048364153627 - nodes in this community are weakly interconnected._
- **Should `Debug Logger (debuglog)` be split into smaller, more focused modules?**
  _Cohesion score 0.1206896551724138 - nodes in this community are weakly interconnected._