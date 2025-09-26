package protocol

import (
	"DS_MP2/internal/membership"
	"DS_MP2/internal/transport"
	mpb "DS_MP2/protoBuilds/membership"
	"context"
	"math/rand"
	"net"
	"time"

	"google.golang.org/protobuf/proto"
)

const budget = 1200

type Protocol struct {
	Table   *membership.Table
	UDP     *transport.UDP
	PQ      *PiggybackQueue
	Logf    func(string, ...interface{})
	FanoutK int
}

func NewProtocol(t *membership.Table, udp *transport.UDP, logf func(string, ...interface{}), fanout int) *Protocol {
	return &Protocol{Table: t, UDP: udp, PQ: NewPiggybackQueue(), Logf: logf, FanoutK: fanout}
}

func (p *Protocol) Handle(ctx context.Context, env *mpb.Envelope, addr *net.UDPAddr) {
	switch env.GetType() {
	case mpb.Envelope_JOIN:
		p.onJoin(ctx, env, addr)
	case mpb.Envelope_JOIN_ACK:
		p.onJoinAck(ctx, env, addr)
	case mpb.Envelope_UPDATE_BATCH:
		p.onUpdateBatch(ctx, env, addr)
	default:
		// ignore
	}
}

func (p *Protocol) onJoin(ctx context.Context, env *mpb.Envelope, addr *net.UDPAddr) {
	j := env.GetJoin()
	if j == nil || j.Node == nil {
		return
	}
	p.Logf("JOIN recv from=%s node=%s",
		addr.String(), membership.StringifyNodeID(j.Node))

	// Apply update
	changed := p.Table.ApplyUpdate(&mpb.MembershipEntry{
		Node:         j.Node,
		State:        mpb.MemberState_ALIVE,
		Incarnation:  j.Node.GetIncarnation(),
		LastUpdateMs: uint64(time.Now().UnixMilli()),
	})

	// Enqueue for piggyback if there was a change
	if changed && p.PQ != nil {
		p.PQ.Enqueue(&mpb.MembershipEntry{
			Node:         j.Node,
			State:        mpb.MemberState_ALIVE,
			Incarnation:  j.Node.GetIncarnation(),
			LastUpdateMs: uint64(time.Now().UnixMilli()),
		})
		p.Logf("JOIN fanout: enqueued node=%s; sending immediate gossip",
			membership.StringifyNodeID(j.Node))
		p.fanoutOnce()
	}

	// Send JoinAck with current membership snapshot
	ack := &mpb.JoinAck{
		Node:               j.Node,
		MembershipSnapshot: p.Table.Snapshot(),
		SentMs:             uint64(time.Now().UnixMilli()),
	}

	// Create response envelope
	resp := &mpb.Envelope{
		Version: 1,
		Sender:  p.Table.GetSelf(),
		Type:    mpb.Envelope_JOIN_ACK,
		Payload: &mpb.Envelope_JoinAck{JoinAck: ack},
	}

	// Send if within budget
	if b, err := proto.Marshal(resp); err == nil && len(b) <= budget {
		_ = p.UDP.Send(addr, resp)
	}
}

func (p *Protocol) onJoinAck(ctx context.Context, env *mpb.Envelope, addr *net.UDPAddr) {
	ack := env.GetJoinAck()
	if ack == nil {
		return
	}

	p.Logf("JOIN_ACK recv from=%s sender=%s snapshot=%d",
		addr.String(), membership.StringifyNodeID(env.GetSender()), len(ack.GetMembershipSnapshot()))

	if n := p.Table.MergeSnapshot(ack.GetMembershipSnapshot()); n > 0 && p.PQ != nil {
		// Optionally enqueue self ALIVE to speed spread
		self := p.Table.GetSelf()
		p.PQ.Enqueue(&mpb.MembershipEntry{
			Node:         self,
			State:        mpb.MemberState_ALIVE,
			Incarnation:  self.GetIncarnation(),
			LastUpdateMs: uint64(time.Now().UnixMilli()),
		})
		p.Logf("JOIN_ACK merge applied=%d", n)
	}

}

func (p *Protocol) onUpdateBatch(ctx context.Context, env *mpb.Envelope, addr *net.UDPAddr) {
	b := env.GetUpdateBatch()
	if b == nil {
		return
	}
	p.Logf("UPDATE_BATCH recv from=%s entries=%d", addr.String(), len(b.GetEntries()))
	for _, e := range b.GetEntries() {
		changed := p.Table.ApplyUpdate(e)
		p.Logf("APPLY origin=gossip from=%s node=%s state=%v inc=%d changed=%v",
			addr.String(), membership.StringifyNodeID(e.Node), e.State, e.Incarnation, changed)
		if changed && p.PQ != nil {
			p.PQ.Enqueue(e)
		}
	}
}

// Helper: choose up to k alive peers (excluding self)
func (p *Protocol) chooseTargets(k int) []*mpb.NodeID {
	selfKey := membership.StringifyNodeID(p.Table.GetSelf())
	var peers []*mpb.NodeID
	for _, m := range p.Table.GetMembers() {
		if m.State != mpb.MemberState_ALIVE {
			continue
		}
		if membership.StringifyNodeID(m.NodeID) == selfKey {
			continue
		}
		peers = append(peers, m.NodeID)
	}
	rand.Shuffle(len(peers), func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })
	if k > len(peers) {
		k = len(peers)
	}
	return peers[:k]
}

func nodeAddr(n *mpb.NodeID) *net.UDPAddr {
	return &net.UDPAddr{IP: net.ParseIP(n.GetIp()), Port: int(n.GetPort())}
}

// Build and send one UPDATE_BATCH within budget to selected targets
func (p *Protocol) fanoutOnce() {
	if p.PQ == nil {
		return
	}
	entries := p.PQ.DrainUpToBytes(budget, func(es []*mpb.MembershipEntry) (int, error) {
		env := &mpb.Envelope{
			Version: 1,
			Sender:  p.Table.GetSelf(),
			Type:    mpb.Envelope_UPDATE_BATCH,
			Payload: &mpb.Envelope_UpdateBatch{UpdateBatch: &mpb.UpdateBatch{Entries: es}},
		}
		b, err := proto.Marshal(env)
		if err != nil {
			return 0, err
		}
		return len(b), nil
	})
	if len(entries) == 0 {
		return
	}
	env := &mpb.Envelope{
		Version: 1,
		Sender:  p.Table.GetSelf(),
		Type:    mpb.Envelope_UPDATE_BATCH,
		Payload: &mpb.Envelope_UpdateBatch{UpdateBatch: &mpb.UpdateBatch{Entries: entries}},
	}
	wire, err := proto.Marshal(env)
	if err != nil || len(wire) > budget {
		return
	}

	p.Logf("GOSSIP fanout entries=%d", len(entries))
	targets := p.chooseTargets(p.FanoutK)
	p.Logf("GOSSIP targets=%d", len(targets))
	for _, n := range targets {
		p.Logf("GOSSIP send to=%s", membership.StringifyNodeID(n))
		_ = p.UDP.Send(nodeAddr(n), env)
	}
}
