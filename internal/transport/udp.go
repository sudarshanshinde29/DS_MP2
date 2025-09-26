package transport

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	mpb "DS_MP2/protoBuilds/membership"

	"google.golang.org/protobuf/proto"
)

type Handler func(ctx context.Context, env *mpb.Envelope, addr *net.UDPAddr)

type UDP struct {
	conn       *net.UDPConn
	handler    Handler
	dropRate   atomic.Value // float64
	recvCount  uint64
	dropCount  uint64
	delivCount uint64
}

func NewUDP(bindAddr string, handler Handler) (*UDP, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", bindAddr)
	if err != nil {
		return nil, err
	}
	c, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	u := &UDP{conn: c, handler: handler}
	u.dropRate.Store(float64(0))
	return u, nil
}

func (u *UDP) SetDropRate(rate float64) {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	u.dropRate.Store(rate)
}

func (u *UDP) Close() error { return u.conn.Close() }

func (u *UDP) Addr() net.Addr { return u.conn.LocalAddr() }

func (u *UDP) Send(to *net.UDPAddr, env *mpb.Envelope) error {
	//  protobuf
	b, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	if len(b) > 1200 {
		return fmt.Errorf("oversize datagram %dB > 1200B", len(b))
	}
	log.Printf("SEND type=%v peer=%s len=%d", env.GetType(), to.String(), len(b))
	_, err = u.conn.WriteToUDP(b, to)
	return err
}

func (u *UDP) Serve(ctx context.Context) error {
	buf := make([]byte, 64*1024)
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		u.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, addr, err := u.conn.ReadFromUDP(buf)
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			continue
		}
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			continue
		}
		// ... receive packet ...
		atomic.AddUint64(&u.recvCount, 1)

		// ARTIFICIAL DROP - this is drop simulation!
		if rnd.Float64() < u.dropRate.Load().(float64) {
			atomic.AddUint64(&u.dropCount, 1)
			continue // <-- DROP the message, don't process it
		}
		var env mpb.Envelope
		if err := proto.Unmarshal(buf[:n], &env); err != nil {
			// drop malformed
			atomic.AddUint64(&u.dropCount, 1)
			continue
		}
		atomic.AddUint64(&u.delivCount, 1)
		if u.handler != nil {
			u.handler(ctx, &env, addr)
		}
	}
}

func (u *UDP) Stats() (recv, dropped, delivered uint64, dropRate float64) {
	return u.recvCount, u.dropCount, u.delivCount, u.dropRate.Load().(float64)
}
