package cli

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"DS_MP2/internal/membership"
	"DS_MP2/internal/transport"
	mpb "DS_MP2/protoBuilds/membership"
)

type CLI struct {
	table     *membership.Table
	transport *transport.UDP
	self      *mpb.NodeID
	logger    func(string, ...interface{})
}

func NewCLI(table *membership.Table, transport *transport.UDP, self *mpb.NodeID, logger func(string, ...interface{})) *CLI {
	return &CLI{
		table:     table,
		transport: transport,
		self:      self,
		logger:    logger,
	}
}

func (c *CLI) HandleCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "list_self":
		c.listSelf()
	case "list_mem":
		c.listMem()
	case "join":
		if len(parts) < 2 {
			fmt.Println("Usage: join <introducer_ip:port>")
			return
		}
		c.join(parts[1])
	case "leave":
		c.leave()
	case "display_suspects":
		c.displaySuspects()
	case "display_protocol":
		c.displayProtocol()
	case "switch":
		if len(parts) < 3 {
			fmt.Println("Usage: switch <gossip|ping> <suspect|nosuspect>")
			return
		}
		c.switchProtocol(parts[1], parts[2])
	default:
		fmt.Printf("Unknown command: %s\n", parts[0])
	}
}

func (c *CLI) listSelf() {
	fmt.Printf("Self: %s\n", membership.StringifyNodeID(c.self))
}

func (c *CLI) listMem() {
	members := c.table.GetMembers()
	fmt.Printf("Membership list (%d members):\n", len(members))
	for _, member := range members {
		fmt.Printf("  %s: state=%v incarnation=%d last_update=%v\n",
			membership.StringifyNodeID(member.NodeID),
			member.State,
			member.Incarnation,
			member.LastUpdate.Format("15:04:05"))
	}
}

func (c *CLI) join(introducerAddr string) {
	// Parse introducer address
	addr, err := parseAddr(introducerAddr)
	if err != nil {
		fmt.Printf("Invalid introducer address: %v\n", err)
		return
	}

	// Create join message
	joinMsg := &mpb.Join{
		Node:   c.self,
		SentMs: uint64(time.Now().UnixMilli()),
	}

	// Create envelope
	env := &mpb.Envelope{
		Version:   1,
		Sender:    c.self,
		Type:      mpb.Envelope_JOIN,
		RequestId: fmt.Sprintf("join_%d", time.Now().UnixNano()),
		Payload:   &mpb.Envelope_Join{Join: joinMsg},
	}

	// Send join request
	if err := c.transport.Send(addr, env); err != nil {
		fmt.Printf("Failed to send join request: %v\n", err)
		return
	}

	c.logger("Sent join request to %s", introducerAddr)
	fmt.Printf("Join request sent to %s\n", introducerAddr)
}

func (c *CLI) leave() {
	// Mark self as LEFT in local table
	entry := &mpb.MembershipEntry{
		Node:         c.self,
		State:        mpb.MemberState_LEFT,
		Incarnation:  c.self.GetIncarnation(),
		LastUpdateMs: uint64(time.Now().UnixMilli()),
	}

	c.table.ApplyUpdate(entry)
	c.logger("Marked self as LEFT")
	fmt.Println("Left the group")
}

func (c *CLI) displaySuspects() {
	// Implement when we add suspicion mechanism

}

func (c *CLI) displayProtocol() {
	// Implement when we add protocol switching
}

func (c *CLI) switchProtocol(mode, suspicion string) {
	// Implement when we add protocol switching

}

func parseAddr(addr string) (*net.UDPAddr, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid address format")
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid port: %v", err)
	}

	return &net.UDPAddr{
		IP:   net.ParseIP(parts[0]),
		Port: port,
	}, nil
}
