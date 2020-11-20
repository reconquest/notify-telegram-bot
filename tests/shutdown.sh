#!/bin/bash

set -euo pipefail

killall notify-telegram-bot 2>/dev/null || true
kill -9 $(pgrep telegram-cli) 2>/dev/null || true
mongo -quiet < drop-db.js >> logs/db.log
