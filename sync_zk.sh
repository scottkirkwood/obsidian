#!/bin/bash

ZK_PATH="/home/scottkirkwood/zk"

cd ${ZK_PATH}

git pull -q

CHANGES_EXIST="$(git status --porcelain | wc -l)"
# Do changes exist?
if [ $CHANGES_EXIST -eq 0 ]; then
  cd - > /dev/null
  exit 0
  echo "exit"
fi

git add .
git commit -q -m "Last Sync: $(date +"%Y-%m-%d %H:%M:%S")"
git push -q

cd - > /dev/null
