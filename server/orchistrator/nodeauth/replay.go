package nodeauth

import "sync"

type ringEntry struct {
	epoch uint32
	seq   uint32
}

type nodeBuf struct {
	buf [32]ringEntry
	idx int
}

// ReplayCache detects replayed messages using a per-node fixed-size ring buffer.
// Each node gets its own 32-slot ring; eviction is FIFO within that node's buffer.
type ReplayCache struct {
	mu    sync.Mutex
	nodes map[[6]byte]*nodeBuf
}

func NewReplayCache(_ int) *ReplayCache {
	return &ReplayCache{nodes: make(map[[6]byte]*nodeBuf)}
}

// IsDuplicate returns true if (mac, epoch, seq) was seen recently; records it otherwise.
// The zero-value check (epoch != 0 || seq != 0) prevents uninitialized buffer slots
// from matching valid messages. Since epoch is a boot counter (starts at 1) and seq is
// a per-boot counter (starts at 1), epoch=0/seq=0 should never appear in valid messages.
func (rc *ReplayCache) IsDuplicate(mac [6]byte, epoch, seq uint32) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	nb := rc.nodes[mac]
	if nb == nil {
		nb = &nodeBuf{}
		rc.nodes[mac] = nb
	}
	for _, e := range nb.buf {
		if e.epoch == epoch && e.seq == seq && (e.epoch != 0 || e.seq != 0) {
			return true
		}
	}
	nb.buf[nb.idx] = ringEntry{epoch: epoch, seq: seq}
	nb.idx = (nb.idx + 1) % 32
	return false
}
