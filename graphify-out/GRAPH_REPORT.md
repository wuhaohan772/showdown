# Graph Report - .  (2026-07-04)

## Corpus Check
- 17 files · ~46,888 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 387 nodes · 754 edges · 32 communities (18 shown, 14 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 92 edges (avg confidence: 0.81)
- Token cost: 37,438 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Agent Adapter & Roster|Agent Adapter & Roster]]
- [[_COMMUNITY_TUI Game Loop|TUI Game Loop]]
- [[_COMMUNITY_Card & Deck Engine|Card & Deck Engine]]
- [[_COMMUNITY_TUI Test Harness|TUI Test Harness]]
- [[_COMMUNITY_Decision Extraction & Fallback|Decision Extraction & Fallback]]
- [[_COMMUNITY_Hand Betting Engine|Hand Betting Engine]]
- [[_COMMUNITY_Hand Engine Tests|Hand Engine Tests]]
- [[_COMMUNITY_Token Cost & Model Choice Plan|Token Cost & Model Choice Plan]]
- [[_COMMUNITY_Match Digest & Talk Log|Match Digest & Talk Log]]
- [[_COMMUNITY_CONTEXT.md Design Concepts|CONTEXT.md Design Concepts]]
- [[_COMMUNITY_Debug Transcript Logger|Debug Transcript Logger]]
- [[_COMMUNITY_Debug Log Spec & Plans|Debug Log Spec & Plans]]
- [[_COMMUNITY_Match & Blind Schedule|Match & Blind Schedule]]
- [[_COMMUNITY_Usage & Response Types|Usage & Response Types]]
- [[_COMMUNITY_Plan Interface Details|Plan Interface Details]]
- [[_COMMUNITY_Asker Logging Wrapper|Asker Logging Wrapper]]
- [[_COMMUNITY_Key Hints Feature Docs|Key Hints Feature Docs]]
- [[_COMMUNITY_Prompt Spike Script|Prompt Spike Script]]
- [[_COMMUNITY_Agent Test Stub|Agent Test Stub]]
- [[_COMMUNITY_Stub Agent Script|Stub Agent Script]]
- [[_COMMUNITY_Poker Action|Poker Action]]
- [[_COMMUNITY_Action Type|Action Type]]
- [[_COMMUNITY_Adapter Symbol|Adapter Symbol]]
- [[_COMMUNITY_Asker Symbol|Asker Symbol]]
- [[_COMMUNITY_Roster Concept|Roster Concept]]
- [[_COMMUNITY_TUI Adapter Ref|TUI Adapter Ref]]
- [[_COMMUNITY_TUI Rand Ref|TUI Rand Ref]]
- [[_COMMUNITY_Logger Symbol|Logger Symbol]]
- [[_COMMUNITY_Main Adapter Ref|Main Adapter Ref]]
- [[_COMMUNITY_Module Root|Module Root]]
- [[_COMMUNITY_Stats Symbol|Stats Symbol]]
- [[_COMMUNITY_Test Symbol|Test Symbol]]

## God Nodes (most connected - your core abstractions)
1. `Model` - 28 edges
2. `Hand` - 25 edges
3. `testModel()` - 20 edges
4. `NewHand()` - 18 edges
5. `key()` - 18 edges
6. `GetDecision()` - 16 edges
7. `Card` - 15 edges
8. `mustApply()` - 13 edges
9. `newTestHand()` - 12 edges
10. `NewModel()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `Cost Tracking` --semantically_similar_to--> `Token/Cost Measurement`  [INFERRED] [semantically similar]
  README.md → docs/superpowers/plans/2026-07-03-token-cost-and-model-choice.md
- `Model Flag (--model)` --semantically_similar_to--> `Model Choice`  [INFERRED] [semantically similar]
  README.md → docs/superpowers/plans/2026-07-03-token-cost-and-model-choice.md
- `main()` --calls--> `DefaultPath()`  [INFERRED]
  main.go → internal/stats/stats.go
- `main()` --calls--> `NewModel()`  [INFERRED]
  main.go → internal/tui/app.go
- `main()` --calls--> `DetectRoster()`  [INFERRED]
  main.go → internal/agent/adapter.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Usage Data Flow** — docs_superpowers_plans_2026_07_03_token_cost_and_model_choice_usage_struct, docs_superpowers_plans_2026_07_03_token_cost_and_model_choice_debug_log, docs_superpowers_plans_2026_07_03_token_cost_and_model_choice_tui_model, docs_superpowers_plans_2026_07_03_token_cost_and_model_choice_stats_record [INFERRED 0.85]
- **Key Hint Row View Phase Switch Implementation** — docs_superpowers_specs_2026_07_03_key_hints_design_key_hint_row, docs_superpowers_specs_2026_07_03_key_hints_design_phase_agent_turn_hint, docs_superpowers_specs_2026_07_03_key_hints_design_phase_hand_end_hint, internal_tui_app_view [EXTRACTED 1.00]
- **Debug Event Capture Pipeline** — docs_superpowers_specs_2026_07_03_debug_transcript_log_design_logger, docs_superpowers_specs_2026_07_03_debug_transcript_log_design_wrapasker, docs_superpowers_specs_2026_07_03_debug_transcript_log_design_apply_helper [EXTRACTED 1.00]
- **Phase-Aware Talk Input Management** — docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_talkreturn_field, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_opentalk_helper, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_phase_widening [EXTRACTED 1.00]
- **Async State Preservation Pattern** — docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_talkreturn_field, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_decision_race_handling, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_runout_tick_handling [EXTRACTED 1.00]
- **Decision Pipeline: Request → Decision → Action** — context_md_decision_request, context_md_decision, context_md_fallback_action [INFERRED 0.85]
- **ADR Implementation in Code** — docs_adr_0001_stateless_decision_requests, docs_adr_0002_native_memory_loading, superpowers_sdd_task_9_report_get_decision, superpowers_sdd_task_10_brief_asker_method [INFERRED 0.75]

## Communities (32 total, 14 thin omitted)

### Community 0 - "Agent Adapter & Roster"
Cohesion: 0.09
Nodes (30): Adapter, Duration, DetectRoster(), T, TestAskerAppliesParse(), TestAskerTimeout(), TestClaudeAdapterHasModelPresets(), TestClaudeAdapterUsesJSONOutput() (+22 more)

### Community 1 - "TUI Game Loop"
Cohesion: 0.12
Nodes (23): Card, Cmd, Digest, Hand, cardStrings(), Action, ActionType, KeyMsg (+15 more)

### Community 2 - "Card & Deck Engine"
Cohesion: 0.12
Nodes (30): Rand, NewDeck(), ParseCard(), T, TestCardString(), TestDeckDeals52Unique(), TestDeckShuffled(), TestParseCardRoundTrip() (+22 more)

### Community 3 - "TUI Test Harness"
Cohesion: 0.22
Nodes (30): NewModel(), KeyMsg, T, key(), readLogEvents(), TestAgentDecisionLogged(), TestAgentDecisionMsgApplies(), TestDebugLogCapturesHandFlow() (+22 more)

### Community 4 - "Decision Extraction & Fallback"
Cohesion: 0.18
Nodes (24): Asker, Decision, Context, ExtractDecision(), FallbackAction(), GetDecision(), GetReaction(), Action (+16 more)

### Community 5 - "Hand Betting Engine"
Cohesion: 0.18
Nodes (7): Action, ActionType, Hand, LogItem, Result, SeatState, Street

### Community 6 - "Hand Engine Tests"
Cohesion: 0.26
Nodes (22): Rand, NewHand(), T, hasAction(), mustApply(), newTestHand(), TestAllInActorHasNoActions(), TestAllInCallRunsOutBoard() (+14 more)

### Community 7 - "Token Cost & Model Choice Plan"
Cohesion: 0.12
Nodes (19): ADR-0001 Invariant, Model Choice, Token/Cost Measurement + Model Choice Implementation Plan, Token/Cost Measurement, ADR-0002, Agent Flag (--agent), Career Stats, Claude Agent (+11 more)

### Community 8 - "Match Digest & Talk Log"
Cohesion: 0.16
Nodes (10): Digest, RequestData, NewDigest(), BuildHandState(), RenderPrompt(), T, TestBuildHandState(), TestBuildHandStatePerspectiveIndependentOfActor() (+2 more)

### Community 9 - "CONTEXT.md Design Concepts"
Cohesion: 0.15
Nodes (17): Agent Adapter, Career Record, Decision, Decision Request, Fallback Action, Game, Hand-End Reaction, Match (+9 more)

### Community 10 - "Debug Transcript Logger"
Cohesion: 0.20
Nodes (13): Logger, File, DefaultPath(), New(), T, readEvents(), TestDefaultPath(), TestLogAfterCloseIsNoop() (+5 more)

### Community 11 - "Debug Log Spec & Plans"
Cohesion: 0.17
Nodes (17): Debug Transcript Log Implementation Plan, Trash-Talk Echo & Phase Widening Implementation Plan, apply Helper, Bug Report Protocol, Debug Transcript Log Design, internal/debuglog Package, JSONL Format, Logger (+9 more)

### Community 12 - "Match & Blind Schedule"
Cohesion: 0.22
Nodes (6): NewMatch(), T, TestBlindSchedule(), TestButtonAlternates(), TestNextHandAndBust(), Match

### Community 13 - "Usage & Response Types"
Cohesion: 0.22
Nodes (9): claudeEnvelope, Response, Usage, ParseClaudeJSON(), T, TestParseClaudeJSON(), TestParseClaudeJSONMalformedFallsBackToRaw(), TestUsageAdd() (+1 more)

### Community 14 - "Plan Interface Details"
Cohesion: 0.23
Nodes (12): Adapter Struct, Asker Type, Claude CLI, Debug Log Usage, GetDecision Function, GetReaction Function, JSON Output Format, ParseClaudeJSON Function (+4 more)

### Community 15 - "Asker Logging Wrapper"
Cohesion: 0.39
Nodes (7): Logger, T, TestWrapAskerLogsError(), TestWrapAskerLogsUsage(), TestWrapAskerNilLogger(), TestWrapAskerPassthroughAndLog(), WrapAsker()

### Community 16 - "Key Hints Feature Docs"
Cohesion: 0.29
Nodes (7): Key Hint Rows Implementation Plan, TestKeyHintRows, Banner Cleanup, Discoverability Gap, Key Hint Row, Phase Agent Turn Hint Case, Phase Hand End Hint Case

### Community 17 - "Prompt Spike Script"
Cohesion: 0.53
Nodes (5): build_prompt(), extract_json(), main(), Find the last parseable JSON object with an 'action' key., run_one()

## Knowledge Gaps
- **34 isolated node(s):** `github.com/haohanwu/showdown`, `stub.sh script`, `stub-agent.sh script`, `Game`, `Roster` (+29 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **14 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Model` connect `TUI Game Loop` to `Agent Adapter & Roster`, `TUI Test Harness`, `Usage & Response Types`?**
  _High betweenness centrality (0.082) - this node is a cross-community bridge._
- **Why does `Hand` connect `Hand Betting Engine` to `Match Digest & Talk Log`, `Card & Deck Engine`, `Hand Engine Tests`?**
  _High betweenness centrality (0.037) - this node is a cross-community bridge._
- **Why does `Adapter` connect `Agent Adapter & Roster` to `TUI Game Loop`, `TUI Test Harness`, `Usage & Response Types`?**
  _High betweenness centrality (0.037) - this node is a cross-community bridge._
- **Are the 12 inferred relationships involving `NewHand()` (e.g. with `TestBuildHandState()` and `TestBuildHandStatePerspectiveIndependentOfActor()`) actually correct?**
  _`NewHand()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/haohanwu/showdown`, `stub.sh script`, `stub-agent.sh script` to the rest of the system?**
  _38 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Agent Adapter & Roster` be split into smaller, more focused modules?**
  _Cohesion score 0.08717948717948718 - nodes in this community are weakly interconnected._
- **Should `TUI Game Loop` be split into smaller, more focused modules?**
  _Cohesion score 0.11605937921727395 - nodes in this community are weakly interconnected._