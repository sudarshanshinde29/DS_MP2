#!/bin/bash


set -e

echo "Installing dependencies..."

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed. Please install Protocol Buffers compiler."
    echo "On Ubuntu: sudo apt-get install protobuf-compiler"
    exit 1
fi

# Check if Go protobuf plugins are installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Generate protobuf code
echo "Generating protobuf code..."
make proto

# Download dependencies
echo "Downloading dependencies..."
go mod tidy

# Build the daemon
echo "Building daemon..."
go build -o daemon ./cmd/daemon

echo "Build completed successfully!"
echo "Run './daemon <ip> <port> [introducer_ip:port]' to start a node"
