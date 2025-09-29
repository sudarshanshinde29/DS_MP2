package main

import (
	"DS_MP2/internal/protocol"
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"DS_MP2/internal/cli"
	"DS_MP2/internal/membership"
	"DS_MP2/internal/transport"
	mpb "DS_MP2/protoBuilds/membership"
)

// DefaultLogDir: set this to a non-empty path to force logs for every node
// into a single directory even when LOG_DIR env is not provided.
// Example: const DefaultLogDir = "/var/tmp/ds_mp2/logs"
const DefaultLogDir = "root/logs"

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ./daemon <ip> <port> [introducer_ip:port]")
		os.Exit(1)
	}

	ip := os.Args[1]
	portStr := os.Args[2]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Invalid port: %v", err)
	}

	// Create self NodeID with current timestamp as incarnation
	incarnation := uint64(time.Now().UnixMilli())
	self, err := membership.NewNodeID(ip, uint32(port), incarnation)
	if err != nil {
		log.Fatalf("Invalid node ID: %v", err)
	}

	// Optional file logging: if LOG_DIR is set, also write logs to LOG_DIR/<ip>_<port>.log
	dir := os.Getenv("LOG_DIR")
	if dir == "" && DefaultLogDir != "" {
		dir = DefaultLogDir
	}
	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
		fname := fmt.Sprintf("failure.log", ip, port)
		fpath := filepath.Join(dir, fname)
		f, err := os.OpenFile(fpath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			// Write to both stdout and file
			log.SetOutput(io.MultiWriter(os.Stdout, f))
			defer f.Close()
		} else {
			fmt.Printf("warning: failed to open log file %s: %v\n", fpath, err)
		}
	}

	// Create logger
	logger := func(format string, args ...interface{}) {
		log.Printf("[%s] %s", membership.StringifyNodeID(self), fmt.Sprintf(format, args...))
	}

	// Create membership table
	table := membership.NewTable(self, logger)

	fanout := 12
	var protoPtr *protocol.Protocol

	handler := func(ctx context.Context, env *mpb.Envelope, addr *net.UDPAddr) {
		if protoPtr != nil {
			protoPtr.Handle(ctx, env, addr)
		}
	}

	// Create UDP transport
	bindAddr := fmt.Sprintf("%s:%d", ip, port)
	var udp *transport.UDP
	udp, err = transport.NewUDP(bindAddr, handler)
	if err != nil {
		log.Fatalf("Failed to create UDP transport: %v", err)
	}
	defer udp.Close()

	protoPtr = protocol.NewProtocol(table, udp, logger, fanout)
	protoPtr.PQ.Enqueue(&mpb.MembershipEntry{
		Node:         table.GetSelf(),
		State:        mpb.MemberState_ALIVE,
		Incarnation:  table.GetSelf().GetIncarnation(),
		LastUpdateMs: uint64(time.Now().UnixMilli()),
	})

	// Start UDP server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := udp.Serve(ctx); err != nil {
			logger("UDP server error: %v", err)
		}
	}()

	cli := cli.NewCLI(table, udp, self, logger, protoPtr)
	// Start gossip
	// Periodic gossip every 300ms with 10% jitter
	protocol.StartGossip(ctx, protoPtr, 300*time.Millisecond, 0.10)

	go protoPtr.StartPingAck(ctx, 300*time.Millisecond, 300*time.Millisecond)

	logger("Daemon started on %s", bindAddr)
	logger("Self: %s", membership.StringifyNodeID(self))

	// Handle introducer join if provided
	if len(os.Args) > 3 {
		introducerAddr := os.Args[3]
		logger("Joining via introducer: %s", introducerAddr)
		cli.HandleCommand(fmt.Sprintf("join %s", introducerAddr))
	}

	// Interactive CLI loop
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for scanner.Scan() {
		cmd := scanner.Text()
		if cmd == "quit" || cmd == "exit" {
			break
		}
		cli.HandleCommand(cmd)
		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		logger("Scanner error: %v", err)
	}

	logger("Daemon shutting down")
}
