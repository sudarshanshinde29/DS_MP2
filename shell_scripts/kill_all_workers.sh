#!/usr/bin/env bash
set -euo pipefail

VM_HOSTNAME="fa25-cs425-10"
COUNT="${COUNT:-10}"

echo "Killing all workers..."

for XX in $(seq -w 01 "$COUNT"); do
  host="${VM_HOSTNAME}${XX}.cs.illinois.edu"
  n=$((10#$XX))  # 01->1
  port=$((8000 + n))  # 8001 -> 8010.
  
  echo "Checking $host (port :$port)..."
  
  # Check if process is listening on the specific port
  process=$(ssh -o ConnectTimeout=10 -o BatchMode=yes -o StrictHostKeyChecking=accept-new root@"$host" "lsof -ti:$port" 2>/dev/null || echo "")
  
  if [ -n "$process" ]; then
    echo "Found process $process on port :$port - killing it..."
    ssh -o ConnectTimeout=10 -o BatchMode=yes -o StrictHostKeyChecking=accept-new root@"$host" "kill $process"
    echo "Process killed on $host (port :$port)"
  else
    echo "No process found on $host (port :$port)"
  fi
done

echo "All workers killed."
