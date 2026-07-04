# Graph Report - .  (2026-07-04)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 395 nodes · 757 edges · 30 communities (18 shown, 12 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 89 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d134a373`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Hand|Hand]]
- [[_COMMUNITY_Model|Model]]
- [[_COMMUNITY_RunPicker|RunPicker]]
- [[_COMMUNITY_Card|Card]]
- [[_COMMUNITY_app_test.go|app_test.go]]
- [[_COMMUNITY_GetDecision|GetDecision]]
- [[_COMMUNITY_Showdown|Showdown]]
- [[_COMMUNITY_Digest|Digest]]
- [[_COMMUNITY_Decision Request|Decision Request]]
- [[_COMMUNITY_Logger|Logger]]
- [[_COMMUNITY_Debug Transcript Log Design|Debug Transcript Log Design]]
- [[_COMMUNITY_Match|Match]]
- [[_COMMUNITY_Usage|Usage]]
- [[_COMMUNITY_Usage Struct|Usage Struct]]
- [[_COMMUNITY_WrapAsker|WrapAsker]]
- [[_COMMUNITY_Key Hint Row|Key Hint Row]]
- [[_COMMUNITY_run_spike.py|run_spike.py]]
- [[_COMMUNITY_TokenCost Measurement + Model Choice Implementation Plan|Token/Cost Measurement + Model Choice Implementation Plan]]
- [[_COMMUNITY_stub.sh|stub.sh]]
- [[_COMMUNITY_stub-agent.sh|stub-agent.sh]]
- [[_COMMUNITY_Asker|Asker]]
- [[_COMMUNITY_Roster|Roster]]
- [[_COMMUNITY_Action|Action]]
- [[_COMMUNITY_ActionType|ActionType]]
- [[_COMMUNITY_Adapter|Adapter]]
- [[_COMMUNITY_Logger|Logger]]
- [[_COMMUNITY_Rand|Rand]]
- [[_COMMUNITY_T|T]]
- [[_COMMUNITY_Adapter|Adapter]]
- [[_COMMUNITY_github.comhaohanwushowdown|github.com/haohanwu/showdown]]

## God Nodes (most connected - your core abstractions)
1. `Model` - 28 edges
2. `Hand` - 25 edges
3. `testModel()` - 21 edges
4. `NewHand()` - 18 edges
5. `key()` - 18 edges
6. `Card` - 15 edges
7. `GetDecision()` - 15 edges
8. `mustApply()` - 13 edges
9. `newTestHand()` - 12 edges
10. `NewModel()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `DefaultPath()`  [INFERRED]
  main.go → internal/stats/stats.go
- `main()` --calls--> `NewModel()`  [INFERRED]
  main.go → internal/tui/app.go
- `main()` --calls--> `DetectRoster()`  [INFERRED]
  main.go → internal/agent/adapter.go
- `main()` --calls--> `Load()`  [INFERRED]
  main.go → internal/stats/stats.go
- `main()` --calls--> `RunPicker()`  [INFERRED]
  main.go → internal/tui/picker.go

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

## Communities (30 total, 12 thin omitted)

### Community 0 - "Hand"
Cohesion: 0.11
Nodes (29): Rand, NewHand(), T, hasAction(), mustApply(), newTestHand(), TestAllInActorHasNoActions(), TestAllInCallRunsOutBoard() (+21 more)

### Community 1 - "Model"
Cohesion: 0.10
Nodes (27): Action, ActionType, Adapter, Card, Cmd, Digest, Hand, cardStrings() (+19 more)

### Community 2 - "RunPicker"
Cohesion: 0.09
Nodes (30): Adapter, Duration, DetectRoster(), T, TestAskerAppliesParse(), TestAskerTimeout(), TestClaudeAdapterHasModelPresets(), TestClaudeAdapterUsesJSONOutput() (+22 more)

### Community 3 - "Card"
Cohesion: 0.12
Nodes (30): Rand, NewDeck(), ParseCard(), T, TestCardString(), TestDeckDeals52Unique(), TestDeckShuffled(), TestParseCardRoundTrip() (+22 more)

### Community 4 - "app_test.go"
Cohesion: 0.21
Nodes (31): NewModel(), KeyMsg, key(), readLogEvents(), TestAgentDecisionLogged(), TestAgentDecisionMsgApplies(), TestDebugLogCapturesHandFlow(), TestFallbackNoticeVisibleInQuietMode() (+23 more)

### Community 5 - "GetDecision"
Cohesion: 0.18
Nodes (24): Asker, Decision, Context, ExtractDecision(), FallbackAction(), GetDecision(), GetReaction(), Action (+16 more)

### Community 6 - "Showdown"
Cohesion: 0.11
Nodes (21): Agent Decision, Agent Memory, Blinds, Career Stats, Chips, Claude, Stateless CLI Invocation, Codex (+13 more)

### Community 7 - "Digest"
Cohesion: 0.16
Nodes (10): Digest, RequestData, NewDigest(), BuildHandState(), RenderPrompt(), T, TestBuildHandState(), TestBuildHandStatePerspectiveIndependentOfActor() (+2 more)

### Community 8 - "Decision Request"
Cohesion: 0.15
Nodes (17): Agent Adapter, Career Record, Decision, Decision Request, Fallback Action, Game, Hand-End Reaction, Match (+9 more)

### Community 9 - "Logger"
Cohesion: 0.20
Nodes (13): Logger, File, DefaultPath(), New(), T, readEvents(), TestDefaultPath(), TestLogAfterCloseIsNoop() (+5 more)

### Community 10 - "Debug Transcript Log Design"
Cohesion: 0.17
Nodes (17): Debug Transcript Log Implementation Plan, Trash-Talk Echo & Phase Widening Implementation Plan, apply Helper, Bug Report Protocol, Debug Transcript Log Design, internal/debuglog Package, JSONL Format, Logger (+9 more)

### Community 11 - "Match"
Cohesion: 0.22
Nodes (6): NewMatch(), T, TestBlindSchedule(), TestButtonAlternates(), TestNextHandAndBust(), Match

### Community 12 - "Usage"
Cohesion: 0.24
Nodes (8): claudeEnvelope, Response, Usage, ParseClaudeJSON(), T, TestParseClaudeJSON(), TestParseClaudeJSONMalformedFallsBackToRaw(), TestUsageAdd()

### Community 13 - "Usage Struct"
Cohesion: 0.23
Nodes (12): Adapter Struct, Asker Type, Claude CLI, Debug Log Usage, GetDecision Function, GetReaction Function, JSON Output Format, ParseClaudeJSON Function (+4 more)

### Community 14 - "WrapAsker"
Cohesion: 0.39
Nodes (7): Logger, T, TestWrapAskerLogsError(), TestWrapAskerLogsUsage(), TestWrapAskerNilLogger(), TestWrapAskerPassthroughAndLog(), WrapAsker()

### Community 15 - "Key Hint Row"
Cohesion: 0.29
Nodes (7): Key Hint Rows Implementation Plan, TestKeyHintRows, Banner Cleanup, Discoverability Gap, Key Hint Row, Phase Agent Turn Hint Case, Phase Hand End Hint Case

### Community 16 - "run_spike.py"
Cohesion: 0.53
Nodes (5): build_prompt(), extract_json(), main(), Find the last parseable JSON object with an 'action' key., run_one()

### Community 17 - "Token/Cost Measurement + Model Choice Implementation Plan"
Cohesion: 0.50
Nodes (4): ADR-0001 Invariant, Model Choice, Token/Cost Measurement + Model Choice Implementation Plan, Token/Cost Measurement

## Knowledge Gaps
- **34 isolated node(s):** `github.com/haohanwu/showdown`, `stub.sh script`, `stub-agent.sh script`, `Game`, `Roster` (+29 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **12 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `RunPicker` to `app_test.go`?**
  _High betweenness centrality (0.091) - this node is a cross-community bridge._
- **Why does `NewModel()` connect `app_test.go` to `Model`, `RunPicker`?**
  _High betweenness centrality (0.088) - this node is a cross-community bridge._
- **Why does `Adapter` connect `RunPicker` to `Usage`?**
  _High betweenness centrality (0.073) - this node is a cross-community bridge._
- **Are the 12 inferred relationships involving `NewHand()` (e.g. with `TestBuildHandState()` and `TestBuildHandStatePerspectiveIndependentOfActor()`) actually correct?**
  _`NewHand()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/haohanwu/showdown`, `stub.sh script`, `stub-agent.sh script` to the rest of the system?**
  _40 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Hand` be split into smaller, more focused modules?**
  _Cohesion score 0.1147086031452359 - nodes in this community are weakly interconnected._
- **Should `Model` be split into smaller, more focused modules?**
  _Cohesion score 0.10188261351052048 - nodes in this community are weakly interconnected._