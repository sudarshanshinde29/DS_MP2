package cli

import (
	"DS_MP2/internal/protocol"
	"fmt"
	"math/rand"
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
	proto     *protocol.Protocol
}

func NewCLI(table *membership.Table, transport *transport.UDP, self *mpb.NodeID, logger func(string, ...interface{}), proto *protocol.Protocol) *CLI {
	return &CLI{
		table:     table,
		transport: transport,
		self:      self,
		logger:    logger,
		proto:     proto,
	}
}

func (c *CLI) displayProtocol() {
	if c.proto == nil {
		fmt.Println("mode=gossip suspicion=off")
		return
	}
	mode := c.proto.Mode()
	sus := "nosuspect"
	if c.proto.SuspicionOn() {
		sus = "suspect"
	}
	fmt.Printf("%s %s\n", mode, sus)
}

func (c *CLI) displaySuspects() {
	members := c.table.GetMembers()
	fmt.Println("Suspected nodes:")
	for _, m := range members {
		if m.State == mpb.MemberState_SUSPECTED {
			fmt.Printf("  %s\n", membership.StringifyNodeID(m.NodeID))
		}
	}
}

func (c *CLI) switchProtocol(mode, suspicion string) {
	if c.proto == nil {
		fmt.Println("protocol not initialized")
		return
	}
	if mode != "gossip" && mode != "ping" {
		fmt.Println("mode must be gossip|ping")
		return
	}
	c.proto.SetMode(mode)
	c.logger("Switched mode to %s", mode)
	switch suspicion {
	case "suspect":
		c.proto.SetSuspicion(true)
		c.logger("Suspicion ON")
	case "nosuspect":
		c.proto.SetSuspicion(false)
		c.logger("Suspicion OFF")
	default:
		fmt.Println("suspicion must be suspect|nosuspect")
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
		prevSus := false
		if c.proto != nil {
			prevSus = c.proto.SuspicionOn()
		}
		c.switchProtocol(parts[1], parts[2])
		// If we just enabled suspicion, initialize grace for peers we haven't heard from
		if c.proto != nil && !prevSus && c.proto.SuspicionOn() {
			c.proto.InitSuspicionGrace()
			// Start a brief grace window to avoid cold-start false SUSPECTs
			c.proto.Sus.StartGrace(2 * time.Second)
			c.logger("Initialized suspicion grace for ALIVE peers")
		}
	case "drop":
		if len(parts) < 2 {
			fmt.Println("Usage: drop <rate 0..1>")
			return
		}
		rate, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || rate < 0 || rate > 1 {
			fmt.Println("drop rate must be a float in [0,1]")
			return
		}
		if c.transport == nil {
			fmt.Println("transport not initialized")
			return
		}
		c.transport.SetDropRate(rate)
		c.logger("Set receive drop rate to %.2f", rate)
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
	// Increment incarnation (safer refutation semantics)
	c.self.Incarnation = c.self.GetIncarnation() + 1

	// Mark self as LEFT locally
	entry := &mpb.MembershipEntry{
		Node:         c.self,
		State:        mpb.MemberState_LEFT,
		Incarnation:  c.self.GetIncarnation(),
		LastUpdateMs: uint64(time.Now().UnixMilli()),
	}
	changed := c.table.ApplyUpdate(entry)
	c.logger("Marked self as LEFT (changed=%v)", changed)
	fmt.Println("Left the group")

	// Immediate fanout to k random ALIVE peers (exclude self)
	k := 3
	var peers []*mpb.NodeID
	for _, m := range c.table.GetMembers() {
		if m.State != mpb.MemberState_ALIVE {
			continue
		}
		if m.NodeID.GetIp() == c.self.GetIp() && m.NodeID.GetPort() == c.self.GetPort() {
			continue
		}
		peers = append(peers, m.NodeID)
	}
	if len(peers) == 0 {
		return
	}
	rand.Shuffle(len(peers), func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })
	if k > len(peers) {
		k = len(peers)
	}

	batch := &mpb.UpdateBatch{Entries: []*mpb.MembershipEntry{entry}}
	env := &mpb.Envelope{
		Version:   1,
		Sender:    c.self,
		Type:      mpb.Envelope_UPDATE_BATCH,
		RequestId: "leavepush",
		Payload:   &mpb.Envelope_UpdateBatch{UpdateBatch: batch},
	}

	for _, n := range peers[:k] {
		dst := &net.UDPAddr{IP: net.ParseIP(n.GetIp()), Port: int(n.GetPort())}
		_ = c.transport.Send(dst, env)
	}

	// Also piggyback the LEFT entry to spread via pings
	if c.proto != nil && c.proto.PQ != nil {
		c.proto.PQ.Enqueue(entry)
	}

	//time.Sleep(300 * time.Millisecond) // one gossip tick
	//os.Exit(0)
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
