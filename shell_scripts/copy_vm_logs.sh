#!/usr/bin/env bash
set -euo pipefail

# ---configurations ---
LOCAL_DIR="MP1 Demo Data FA22"     # folder containing vm1.log, vm2.log, ... vm10.log
REMOTE_DIR="/root/logs"            # remote folder where we need to copy the log file
VM_HOSTNAME="fa25-cs425-10"

# for each VM copy log file
for i in $(seq -w 01 10); do
  host="${VM_HOSTNAME}${i}.cs.illinois.edu"
  n=$((10#$i))                     # 01->1, 02->2, ... 10->10
  logfile="vm${n}.log"
  src="${LOCAL_DIR}/${logfile}"

  if [[ ! -f "$src" ]]; then
    echo "[SKIP] $src not found"
    continue
  fi

  echo ">>> ${logfile} -> root@${host}:${REMOTE_DIR}/"
  # make sure remote directory exists
  ssh -o BatchMode=yes root@"$host" "mkdir -p '$REMOTE_DIR'"
  # Copy the file
  scp -q "$src" root@"$host":"$REMOTE_DIR"/
  echo "OK: Successfully copied ${logfile} to ${host}:${REMOTE_DIR}/"
done

echo "Done COPYING log files."
