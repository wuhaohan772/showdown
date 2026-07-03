#!/bin/sh
# Fake opponent for end-to-end testing: checks when legal, else calls.
legal=$(printf '%s' "$1" | grep '^Legal actions:')
case "$legal" in
  *check*) echo '{"action": "check", "say": "stub checks"}' ;;
  *)       echo '{"action": "call", "say": "stub calls"}' ;;
esac
