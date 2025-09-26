package protocol

import (
	"context"
	"time"
)

func StartGossip(ctx context.Context, p *Protocol, period time.Duration, jitterFrac float64) (stop func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		randJitter := func() time.Duration {
			j := 1 + (jitterFrac*(0.5-float64(time.Now().UnixNano()%1000)/1000.0))*2
			if j < 0.5 {
				j = 0.5
			}
			if j > 1.5 {
				j = 1.5
			}
			return time.Duration(j * float64(period))
		}
		timer := time.NewTimer(randJitter())
		defer timer.Stop()
		for {
			select {
			// stop when the parent context is canceled
			case <-ctx.Done():
				return
			// on each tick, call p.fanoutOnce() to send one UPDATE_BATCH to k random peers, then reset timer with a new jittered period.
			case <-timer.C:
				p.fanoutOnce()
				timer.Reset(randJitter())
			}
		}
	}()
	return func() { /* cancel via ctx */ }
}
