import json
import os
import tempfile
import unittest

from plot_token_usage import parse_agent_calls, parse_stats


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


class TestParseStats(unittest.TestCase):
    def _write_json(self, obj):
        f = tempfile.NamedTemporaryFile(
            mode="w", suffix=".json", delete=False
        )
        json.dump(obj, f)
        f.close()
        self.addCleanup(os.unlink, f.name)
        return f.name

    def test_filters_to_requested_keys(self):
        path = self._write_json(
            {
                "haiku": {
                    "wins": 3,
                    "losses": 1,
                    "tokens_in": 10000,
                    "tokens_out": 2000,
                    "cost_usd": 0.05,
                },
                "sonnet": {
                    "wins": 1,
                    "losses": 2,
                    "tokens_in": 8000,
                    "tokens_out": 1500,
                    "cost_usd": 0.40,
                },
                "codex": {"wins": 1, "losses": 0},
            }
        )
        result = parse_stats(path, ["haiku", "sonnet"])
        self.assertEqual(set(result.keys()), {"haiku", "sonnet"})
        self.assertEqual(result["haiku"]["tokens_in"], 10000)
        self.assertEqual(result["sonnet"]["cost_usd"], 0.40)

    def test_omits_keys_absent_from_file(self):
        path = self._write_json({"haiku": {"tokens_in": 100}})
        result = parse_stats(path, ["haiku", "sonnet"])
        self.assertEqual(set(result.keys()), {"haiku"})


if __name__ == "__main__":
    unittest.main()
