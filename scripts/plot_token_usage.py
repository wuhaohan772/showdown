#!/usr/bin/env python3
"""Generate token-usage charts for the README from real showdown match data.

Requires: pip install -r requirements.txt

Usage:
  python3 plot_token_usage.py --debug ~/.showdown/debug-<ts>.jsonl \
      --stats ~/.showdown/stats.json --out ../docs/
"""

import argparse
import json


def parse_agent_calls(jsonl_path):
    """Read one match's --debug JSONL, return agent_call records sorted by seq."""
    records = []
    with open(jsonl_path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rec = json.loads(line)
            if rec.get("event") != "agent_call":
                continue
            records.append(
                {
                    "seq": rec["seq"],
                    "tokens_in": rec.get("tokens_in", 0),
                    "tokens_out": rec.get("tokens_out", 0),
                    "cache_read": rec.get("cache_read", 0),
                    "cache_write": rec.get("cache_write", 0),
                    "cost_usd": rec.get("cost_usd", 0.0),
                }
            )
    records.sort(key=lambda r: r["seq"])
    return records
