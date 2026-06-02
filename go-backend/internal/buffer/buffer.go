package buffer

import (
	"sync"

	"predictor/internal/schemas"
)

// RingBuffer is a thread-safe circular buffer for storing candles
type RingBuffer struct {
	mu       sync.RWMutex
	data     []schemas.Candle
	capacity int
	size     int
	head     int
	tail     int
}

// New creates a new RingBuffer with the specified capacity
// Returns nil if capacity is zero or negative
func New(capacity int) *RingBuffer {
	if capacity <= 0 {
		return nil
	}
	
	return &RingBuffer{
		data:     make([]schemas.Candle, capacity),
		capacity: capacity,
		size:     0,
		head:     0,
		tail:     0,
	}
}

// Append adds a candle to the buffer
// If buffer is at capacity, the oldest candle is evicted
func (rb *RingBuffer) Append(candle schemas.Candle) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	
	rb.data[rb.tail] = candle
	rb.tail = (rb.tail + 1) % rb.capacity
	
	if rb.size < rb.capacity {
		rb.size++
	} else {
		// Buffer is full, move head to evict oldest
		rb.head = (rb.head + 1) % rb.capacity
	}
}

// Snapshot returns a copy of all candles in chronological order
// The returned slice is a deep copy - modifications won't affect the buffer
func (rb *RingBuffer) Snapshot() []schemas.Candle {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	
	if rb.size == 0 {
		return []schemas.Candle{}
	}
	
	result := make([]schemas.Candle, rb.size)
	for i := 0; i < rb.size; i++ {
		idx := (rb.head + i) % rb.capacity
		result[i] = rb.data[idx]
	}
	
	return result
}

// Len returns the current number of candles in the buffer
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// IsFull returns true when the buffer has reached its maximum capacity
func (rb *RingBuffer) IsFull() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size == rb.capacity
}

// Clear removes all candles from the buffer
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.size = 0
	rb.head = 0
	rb.tail = 0
}
