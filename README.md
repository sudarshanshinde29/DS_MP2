# Distributed Systems Membership Protocol (DS_MP2)

A Go-based implementation of a distributed membership protocol for managing node membership in a distributed system. The project implements both Gossip-style heartbeating and Ping-Ack (SWIM-style probing), each with an optional Suspicion mechanism, plus node joining, leaving, and failure detection.

## Features

- **Node Management**: Join and leave operations for distributed nodes
- **Membership Table**: Thread-safe membership tracking with state management and GC of terminal states
- **UDP Transport**: UDP messaging with configurable receive drop rate (for experiments)
- **Protobuf Communication**: Efficient binary serialization for network messages
- **CLI Interface**: Interactive command-line interface for testing and experiments

## Architecture

```
├── cmd/daemon/          # Main application entry point
├── internal/
│   ├── cli/            # Command-line interface and commands
│   ├── membership/     # Membership table, states, GC, snapshots
│   ├── protocol/       # Gossip, Ping-Ack, handlers, suspicion manager, piggyback
│   └── transport/      # UDP transport (drop injection for receive path)
├── proto/              # Protocol buffer definitions
└── protoBuilds/        # Generated protobuf code
```

## Prerequisites

- Go 1.21 or later
- Protocol Buffers compiler (`protoc`)
- Go protobuf plugins:
  - `protoc-gen-go`
  - `protoc-gen-go-grpc`

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd DS_MP2
```

2. Install dependencies:
```bash
go mod tidy
```

3. Generate protobuf code:
```bash
make proto
```

4. Build the daemon:
```bash
make build
```

## Usage

### Starting a Node

```bash
./daemon <ip> <port> [introducer_ip:port]
```

**Examples:**
```bash
# Start first node (no introducer)
./daemon 127.0.0.1 6001

# Start node and join via introducer
./daemon 127.0.0.1 6002 127.0.0.1:6001
```

### Logging
- By default logs go to stdout. To additionally write to a file, set environment variable `LOG_DIR` (or hard-code `DefaultLogDir` in `cmd/daemon/main.go`).
- Each node writes to `<LOG_DIR>/<ip>_<port>.log` while still printing to the terminal.

### Available Commands

Once the daemon is running, you can use these CLI commands:

- `list_self` — Display current node information
- `list_mem` — List all members in the membership table
- `join <introducer_ip:port>` — Join the group via an introducer
- `leave` — Voluntarily leave (LEFT state with higher incarnation; fanned out)
- `switch <gossip|ping> <suspect|nosuspect>` — Switch dissemination and suspicion mode
- `display_protocol` — Print current mode and suspicion flag
- `display_suspects` — Print locally SUSPECTED nodes
- `drop <rate>` — Set per-process receive drop rate in [0,1] (for experiments)

### Example Session

```bash
$ ./daemon 127.0.0.1 6001
[127.0.0.1:6001#1703123456789] Daemon started on 127.0.0.1:6001
[127.0.0.1:6001#1703123456789] Self: 127.0.0.1:6001#1703123456789
> display_protocol
ping nosuspect
> switch gossip suspect
> list_mem
...
```

## Configuration

- The `cluster.properties` file contains an example topology. Most behavior is controlled at runtime via CLI commands.
- Key timers: set in `internal/protocol/handlers.go` via `protocol.NewProtocol` (defaults: period≈300ms; suspicion Tfail≈1s, Tcleanup≈1s). Adjust only if required by experiments.