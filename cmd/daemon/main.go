package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	mpb "DS_MP2/protoBuilds/membership"
	"DS_MP2/internal/cli"
	"DS_MP2/internal/membership"
	"DS_MP2/internal/transport"
)

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

	// Create logger
	logger := func(format string, args ...interface{}) {
		log.Printf("[%s] %s", membership.StringifyNodeID(self), fmt.Sprintf(format, args...))
	}

	// Create membership table
	table := membership.NewTable(self, logger)

	// Create UDP transport
	bindAddr := fmt.Sprintf("%s:%d", ip, port)
	var udp *transport.UDP
	udp, err = transport.NewUDP(bindAddr, func(ctx context.Context, env *mpb.Envelope, addr *net.UDPAddr) {
		handleMessage(table, udp, env, addr, logger)
	})
	if err != nil {
		log.Fatalf("Failed to create UDP transport: %v", err)
	}
	defer udp.Close()

	// Create CLI
	cli := cli.NewCLI(table, udp, self, logger)

	// Start UDP server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := udp.Serve(ctx); err != nil {
			logger("UDP server error: %v", err)
		}
	}()

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

func handleMessage(table *membership.Table, transport *transport.UDP, env *mpb.Envelope, addr *net.UDPAddr, logger func(string, ...interface{})) {
	logger("Received message from %s: type=%v", addr, env.GetType())

	switch env.GetType() {
	case mpb.Envelope_JOIN:
		handleJoin(table, transport, env, addr, logger)
	case mpb.Envelope_JOIN_ACK:
		handleJoinAck(table, env, addr, logger)
	case mpb.Envelope_UPDATE_BATCH:
		handleUpdateBatch(table, env, addr, logger)
	default:
		logger("Unknown message type: %v", env.GetType())
	}
}

func handleJoin(table *membership.Table, transport *transport.UDP, env *mpb.Envelope, addr *net.UDPAddr, logger func(string, ...interface{})) {
	join := env.GetJoin()
	if join == nil {
		return
	}

	logger("Processing join from %s", membership.StringifyNodeID(join.Node))

	// Add joining node to membership table
	entry := &mpb.MembershipEntry{
		Node:          join.Node,
		State:         mpb.MemberState_ALIVE,
		Incarnation:   join.Node.GetIncarnation(),
		LastUpdateMs:  uint64(time.Now().UnixMilli()),
	}
	table.ApplyUpdate(entry)

	// Send join ack with current membership snapshot
	members := table.GetMembers()
	joinAck := &mpb.JoinAck{
		Node:                 join.Node,
		MembershipSnapshot:   make([]*mpb.MembershipEntry, len(members)),
		SentMs:              uint64(time.Now().UnixMilli()),
	}

	for i, member := range members {
		joinAck.MembershipSnapshot[i] = &mpb.MembershipEntry{
			Node:          member.NodeID,
			State:         member.State,
			Incarnation:   member.Incarnation,
			LastUpdateMs:  uint64(member.LastUpdate.UnixMilli()),
		}
	}

	// Create response envelope
	response := &mpb.Envelope{
		Version:   1,
		Sender:    table.GetSelf(),
		Type:      mpb.Envelope_JOIN_ACK,
		RequestId: env.GetRequestId(),
		Payload:   &mpb.Envelope_JoinAck{JoinAck: joinAck},
	}

	// Send response back to sender
	if err := transport.Send(addr, response); err != nil {
		logger("Failed to send join ack: %v", err)
	} else {
		logger("Sent join ack to %s with %d members", addr, len(members))
	}
}

func handleJoinAck(table *membership.Table, env *mpb.Envelope, addr *net.UDPAddr, logger func(string, ...interface{})) {
	joinAck := env.GetJoinAck()
	if joinAck == nil {
		return
	}

	logger("Received join ack with %d members", len(joinAck.MembershipSnapshot))

	// Apply all membership entries from snapshot
	for _, entry := range joinAck.MembershipSnapshot {
		table.ApplyUpdate(entry)
	}

	logger("Updated membership table with snapshot")
}

func handleUpdateBatch(table *membership.Table, env *mpb.Envelope, addr *net.UDPAddr, logger func(string, ...interface{})) {
	updateBatch := env.GetUpdateBatch()
	if updateBatch == nil {
		return
	}

	logger("Received update batch with %d entries", len(updateBatch.Entries))

	// Apply all updates
	for _, entry := range updateBatch.Entries {
		table.ApplyUpdate(entry)
	}
}
