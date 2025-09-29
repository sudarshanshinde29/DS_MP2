#!/usr/bin/env bash
set -euo pipefail

USER="root"
BASE="fa25-cs425-10"

# Everything inside single quotes is sent literally to the remote.
# Use \$ to ensure $... expands on the remote, not locally.
CMD='
  set -e
  cd /root/MP/DS_MP2
  export GOTOOLCHAIN=auto
  # Set GOPATH if missing, on the REMOTE
  export GOPATH="${GOPATH:-$(go env GOPATH)}"
  # Or if you want to be extra safe about remote expansion, escape $:  "${GOPATH:-\$(go env GOPATH)}"
  export PATH="$PATH:$GOPATH/bin"

  echo "REMOTE GOPATH=$GOPATH"
  echo "REMOTE PATH=$PATH"

  git pull
  go mod tidy
  make proto
  make build
'

for i in $(seq -w 01 10); do
  HOST="${BASE}${i}.cs.illinois.edu"
  echo "=== $HOST ==="
  ssh -o BatchMode=yes "$USER@$HOST" "$CMD"
done
