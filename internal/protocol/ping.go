package protocol

import (
	"DS_MP2/internal/membership"
	mpb "DS_MP2/protoBuilds/membership"
	"context"
	"time"

	"google.golang.org/protobuf/proto"
)

func (p *Protocol) StartPingAck(ctx context.Context, period time.Duration, ackMs time.Duration) {
	t := time.NewTimer(period)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if p.Mode() != "ping" { // only active in ping mode
				t.Reset(period)
				continue
			}
			// pick one target
			targets := p.chooseTargets(1)
			if len(targets) == 0 {
				t.Reset(period)
				continue
			}
			dst := targets[0]
			// drain piggyback under budget
			env := p.buildUpdateBatchEnvelope() // may be nil if no entries
			// build PING with piggyback optionally attached (we attach neither in proto; just send UPDATE first if present)
			if env != nil {
				_ = p.UDP.Send(nodeAddr(dst), env)
			}
			p.sendPING(dst)

			// wait for ACK
			key := membership.StringifyNodeID(dst)

			sentAt := time.Now()
			timer := time.NewTimer(ackMs)
			select {
			case <-timer.C:
				if p.SuspicionOn() && !p.Sus.HeardSince(key, sentAt) {
					if e := p.Sus.OnNoAck(key, dst, time.Now()); e != nil {
						p.PQ.Enqueue(e)
						p.fanoutOnce()
					}
				}
			case <-ctx.Done():
				timer.Stop()
				return
			}
			timer.Stop()
			t.Reset(period)
		}
	}
}

func (p *Protocol) sendPING(dst *mpb.NodeID) {
	env := &mpb.Envelope{
		Version: 1,
		Sender:  p.Table.GetSelf(),
		Type:    mpb.Envelope_PING,
	}
	_ = p.UDP.Send(nodeAddr(dst), env)
}

func (p *Protocol) onPing(dst *mpb.NodeID, from *mpb.NodeID) {
	// reply Ack, include piggyback if any
	env := p.buildUpdateBatchEnvelope()
	if env != nil {
		_ = p.UDP.Send(nodeAddr(from), env)
	}
	ack := &mpb.Envelope{Version: 1, Sender: p.Table.GetSelf(), Type: mpb.Envelope_ACK}
	_ = p.UDP.Send(nodeAddr(from), ack)
}

func (p *Protocol) onACK(from *mpb.NodeID) {
	// mark heard; any waiting goroutine will see this through Suspicion path
	if p.SuspicionOn() {
		p.Sus.OnHearFrom(membership.StringifyNodeID(from), time.Now())
	}
}

func (p *Protocol) buildUpdateBatchEnvelope() *mpb.Envelope {
	entries := p.PQ.DrainUpToBytes(budget, func(es []*mpb.MembershipEntry) (int, error) {
		e := &mpb.Envelope{Version: 1, Sender: p.Table.GetSelf(), Type: mpb.Envelope_UPDATE_BATCH,
			Payload: &mpb.Envelope_UpdateBatch{UpdateBatch: &mpb.UpdateBatch{Entries: es}}}
		b, err := proto.Marshal(e)
		if err != nil {
			return 0, err
		}
		return len(b), nil
	})
	if len(entries) == 0 {
		return nil
	}
	return &mpb.Envelope{
		Version: 1,
		Sender:  p.Table.GetSelf(),
		Type:    mpb.Envelope_UPDATE_BATCH,
		Payload: &mpb.Envelope_UpdateBatch{UpdateBatch: &mpb.UpdateBatch{Entries: entries}},
	}
}
