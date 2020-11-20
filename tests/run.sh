#!/bin/bash

set -euo pipefail

./setup.sh

sleep 1s
echo "::tests::prepare telegram-cli"
nc localhost 8080 >/dev/null <<< "dialog_list" &

sleep 3s
echo "::tests::testcase 1"
nc localhost 8080 >> logs/nc.log < subscribe.command &

sleep 1s
echo "::tests::db find endpoints"
mongo --quiet < endpoints-find.js >> logs/db.log

echo "::tests::wait 5s"
sleep 5s

echo "::tests::shutdown"
./shutdown.sh

cat logs/run.log
