import json
import os
import tempfile
import unittest

from plot_token_usage import parse_agent_calls


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


if __name__ == "__main__":
    unittest.main()
