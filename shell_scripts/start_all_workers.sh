#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="/root/MP/DS_MP1"
VM_HOSTNAME="fa25-cs425-10"    #fa25-cs425-1001.cs.illinois.edu
COUNT="${COUNT:-10}"

deploy_host() {
  local host="$1"
  local n="$2"
  local port=$((8000 + n))         # 8001..8010
  local label="vm${n}"
  local glob="vm${n}.log"
  local out="/root/worker_${n}.log"

  echo "[$host] deploy: label=$label port=$port glob=$glob"

  ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new root@"$host"  << EOF
    echo "Connected to $host"
            cd $REPO_DIR
            echo "Changed directory to $REPO_DIR"
            echo "Starting worker..."
            export GOTOOLCHAIN=auto
            go mod tidy
            nohup go run worker/main.go -addr ":$port" -logdir /root/logs -glob "$glob" -label "$label" > "$out" 2>&1 &
EOF
        echo "Disconnected from $host"
        echo "------------------------"
  echo "Started: $label on :$port (log -> $out)"
}


# for each VM , host deployment 
for XX in $(seq -w 01 "$COUNT"); do
  host="${VM_HOSTNAME}${XX}.cs.illinois.edu"
  n=$((10#$XX))  # 01->10
  deploy_host "$host" "$n" &
done
wait
echo "All workers deployed."
