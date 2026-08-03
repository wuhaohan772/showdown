import json
import os
import tempfile
import unittest

from plot_token_usage import (
    parse_agent_calls,
    read_match_model,
    summarize_match,
    render_per_hand_chart,
    render_cross_model_chart,
)


class TestParseAgentCalls(unittest.TestCase):
    def _write_jsonl(self, lines):
        f = tempfile.NamedTemporaryFile(
            mode="w", suffix=".jsonl", delete=False
        )
        for line in lines:
            f.write(json.dumps(line) + "\n")
        f.close()
        self.addCleanup(os.unlink, f.name)
        return f.name

    def test_extracts_and_orders_agent_call_events(self):
        path = self._write_jsonl(
            [
                {
                    "seq": 2,
                    "event": "agent_call",
                    "tokens_in": 500,
                    "tokens_out": 80,
                    "cache_read": 400,
                    "cache_write": 0,
                    "cost_usd": 0.002,
                },
                {"seq": 1, "event": "hand_start"},
                {
                    "seq": 3,
                    "event": "agent_call",
                    "tokens_in": 2000,
                    "tokens_out": 90,
                    "cache_read": 0,
                    "cache_write": 1900,
                    "cost_usd": 0.02,
                },
            ]
        )
        result = parse_agent_calls(path)
        self.assertEqual([r["seq"] for r in result], [2, 3])
        self.assertEqual(result[0]["tokens_in"], 500)
        self.assertEqual(result[0]["cache_read"], 400)
        self.assertEqual(result[1]["cache_write"], 1900)

    def test_defaults_missing_numeric_fields_to_zero(self):
        path = self._write_jsonl(
            [{"seq": 1, "event": "agent_call"}]
        )
        result = parse_agent_calls(path)
        self.assertEqual(
            result[0],
            {
                "seq": 1,
                "tokens_in": 0,
                "tokens_out": 0,
                "cache_read": 0,
                "cache_write": 0,
                "cost_usd": 0.0,
            },
        )


class TestReadMatchModel(unittest.TestCase):
    def _write_jsonl(self, lines):
        f = tempfile.NamedTemporaryFile(
            mode="w", suffix=".jsonl", delete=False
        )
        for line in lines:
            f.write(json.dumps(line) + "\n")
        f.close()
        self.addCleanup(os.unlink, f.name)
        return f.name

    def test_reads_model_from_session_start(self):
        path = self._write_jsonl(
            [
                {"seq": 1, "event": "session_start", "model": "sonnet"},
                {"seq": 2, "event": "agent_call", "tokens_in": 100},
            ]
        )
        self.assertEqual(read_match_model(path), "sonnet")

    def test_returns_unknown_when_no_session_start(self):
        path = self._write_jsonl(
            [{"seq": 1, "event": "agent_call", "tokens_in": 100}]
        )
        self.assertEqual(read_match_model(path), "unknown")


class TestSummarizeMatch(unittest.TestCase):
    def _write_jsonl(self, lines):
        f = tempfile.NamedTemporaryFile(
            mode="w", suffix=".jsonl", delete=False
        )
        for line in lines:
            f.write(json.dumps(line) + "\n")
        f.close()
        self.addCleanup(os.unlink, f.name)
        return f.name

    def test_sums_tokens_and_cost_across_calls(self):
        path = self._write_jsonl(
            [
                {"seq": 1, "event": "session_start", "model": "haiku"},
                {
                    "seq": 2,
                    "event": "agent_call",
                    "tokens_in": 2000,
                    "tokens_out": 90,
                    "cache_read": 0,
                    "cache_write": 1900,
                    "cost_usd": 0.02,
                },
                {
                    "seq": 3,
                    "event": "agent_call",
                    "tokens_in": 2100,
                    "tokens_out": 85,
                    "cache_read": 2000,
                    "cache_write": 0,
                    "cost_usd": 0.003,
                },
            ]
        )
        result = summarize_match(path)
        self.assertEqual(
            result,
            {"tokens_in": 4100, "tokens_out": 175, "cost_usd": 0.023},
        )

    def test_empty_file_sums_to_zero(self):
        path = self._write_jsonl(
            [{"seq": 1, "event": "session_start", "model": "sonnet"}]
        )
        result = summarize_match(path)
        self.assertEqual(
            result, {"tokens_in": 0, "tokens_out": 0, "cost_usd": 0.0}
        )


class TestRenderPerHandChart(unittest.TestCase):
    def test_writes_nonempty_png(self):
        calls = [
            {"seq": 1, "tokens_in": 2000, "tokens_out": 90,
             "cache_read": 0, "cache_write": 1900, "cost_usd": 0.02},
            {"seq": 2, "tokens_in": 2100, "tokens_out": 85,
             "cache_read": 2000, "cache_write": 0, "cost_usd": 0.003},
        ]
        out = tempfile.NamedTemporaryFile(suffix=".png", delete=False)
        out.close()
        self.addCleanup(os.unlink, out.name)

        render_per_hand_chart(calls, out.name)

        self.assertTrue(os.path.getsize(out.name) > 0)


class TestRenderCrossModelChart(unittest.TestCase):
    def test_writes_nonempty_png(self):
        stats = {
            "haiku": {"tokens_in": 10000, "tokens_out": 2000, "cost_usd": 0.05},
            "sonnet": {"tokens_in": 8000, "tokens_out": 1500, "cost_usd": 0.40},
        }
        out = tempfile.NamedTemporaryFile(suffix=".png", delete=False)
        out.close()
        self.addCleanup(os.unlink, out.name)

        render_cross_model_chart(stats, out.name)

        self.assertTrue(os.path.getsize(out.name) > 0)


if __name__ == "__main__":
    unittest.main()
