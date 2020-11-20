#!/bin/bash

set -euo pipefail

mkdir -p logs

echo "::tests::clean logs"
echo -n > logs/run.log
echo -n > logs/db.log
echo -n > logs/nc.log

echo "::tests::build"
$(cd .. && go build)

echo "::tests::run binary"
$(cd .. && ./notify-telegram-bot > tests/logs/run.log 2>&1) &

echo "::tests::run telegram-cli"
telegram-cli --daemonize --tcp-port 8080 &
