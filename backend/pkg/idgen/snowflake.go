package idgen

import (
	"sync"
	"time"
)

// Simple snowflake: 41bit ts | 10bit node | 12bit seq
type Generator struct {
	mu       sync.Mutex
	epoch    int64
	nodeID   int64
	sequence int64
	ts       int64
}

func New(nodeID int64) *Generator {
	return &Generator{epoch: 1704067200000, nodeID: nodeID & 0x3FF}
}

func (g *Generator) Next() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().UnixMilli() - g.epoch
	if now == g.ts {
		g.sequence = (g.sequence + 1) & 0xFFF
		if g.sequence == 0 {
			for now <= g.ts {
				now = time.Now().UnixMilli() - g.epoch
			}
		}
	} else {
		g.sequence = 0
	}
	g.ts = now
	return (now << 22) | (g.nodeID << 12) | g.sequence
}

func DirectConversationID(a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return formatPair(a, b)
}

func formatPair(a, b int64) string {
	return itoa(a) + "_" + itoa(b)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
