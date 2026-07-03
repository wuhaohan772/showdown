#!/bin/sh
# Fake opponent for end-to-end testing: calls everything, checks when free.
# Crude: if the prompt lists "check" as legal, check; else call.
case "$1" in
  *'check'*) echo '{"action": "check", "say": "stub checks"}' ;;
  *)         echo '{"action": "call", "say": "stub calls"}' ;;
esac
