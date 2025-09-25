# Distributed Systems Membership Protocol (DS_MP2)

A Go-based implementation of a distributed membership protocol for managing node membership in a distributed system. This project implements a gossip-based membership protocol with support for node joining, leaving, and failure detection.

## Features

- **Node Management**: Join and leave operations for distributed nodes
- **Membership Table**: Thread-safe membership tracking with state management
- **UDP Transport**: Reliable UDP-based communication with configurable drop rates
- **Protobuf Communication**: Efficient binary serialization for network messages
- **CLI Interface**: Interactive command-line interface for testing and debugging

## Architecture

```
├── cmd/daemon/          # Main application entry point
├── internal/
│   ├── cli/            # Command-line interface
│   ├── membership/     # Membership table and node management
│   └── transport/      # UDP transport layer
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

### Available Commands

Once the daemon is running, you can use these CLI commands:

- `list_self` - Display current node information
- `list_mem` - List all members in the membership table
- `join <introducer_ip:port>` - Join the group via an introducer
- `leave` - Leave the group

### Example Session

```bash
$ ./daemon 127.0.0.1 6001
[127.0.0.1:6001#1703123456789] Daemon started on 127.0.0.1:6001
[127.0.0.1:6001#1703123456789] Self: 127.0.0.1:6001#1703123456789
> list_self
Self: 127.0.0.1:6001#1703123456789
> list_mem
Membership list (1 members):
  127.0.0.1:6001#1703123456789: state=ALIVE incarnation=1703123456789 last_update=14:30:56
```

## Configuration

The `cluster.properties` file contains configuration for a 10-node cluster setup. You can modify this file to change the cluster topology.

