package buffer

import (
	"testing"
	"time"

	"predictor/internal/schemas"
)

// TestAppendWithinCapacity verifies that appending within capacity works correctly
func TestAppendWithinCapacity(t *testing.T) {
	buf := New(60)
	
	// Append 30 candles
	for i := 0; i < 30; i++ {
		candle := schemas.Candle{
			Symbol:    "BTCUSDT",
			Interval:  "1m",
			Open:      float64(i),
			High:      float64(i) + 1,
			Low:       float64(i) - 1,
			Close:     float64(i) + 0.5,
			Volume:    float64(i) * 100,
			CloseTime: time.Now(),
			Timestamp: time.Now(),
		}
		buf.Append(candle)
	}
	
	if buf.Len() != 30 {
		t.Errorf("Expected Len() = 30, got %d", buf.Len())
	}
	
	if buf.IsFull() {
		t.Error("Expected IsFull() = false for 30/60 capacity")
	}
}

// TestAppendExceedsCapacity verifies that oldest candles are evicted when capacity exceeded
func TestAppendExceedsCapacity(t *testing.T) {
	buf := New(60)
	
	// Append 61 candles (1 more than capacity)
	for i := 0; i < 61; i++ {
		candle := schemas.Candle{
			Symbol:    "BTCUSDT",
			Interval:  "1m",
			Open:      float64(i),
			High:      float64(i) + 1,
			Low:       float64(i) - 1,
			Close:     float64(i) + 0.5,
			Volume:    float64(i) * 100,
			CloseTime: time.Now(),
			Timestamp: time.Now(),
		}
		buf.Append(candle)
	}
	
	if buf.Len() != 60 {
		t.Errorf("Expected Len() = 60 after evicting, got %d", buf.Len())
	}
	
	if !buf.IsFull() {
		t.Error("Expected IsFull() = true for 60/60 capacity")
	}
	
	// Verify oldest was evicted (first candle should have Open=1, not 0)
	snapshot := buf.Snapshot()
	if len(snapshot) == 0 {
		t.Fatal("Snapshot is empty")
	}
	
	// First candle should now have Open=1 (the second one appended)
	if snapshot[0].Open != 1 {
		t.Errorf("Expected first candle Open=1 after eviction, got %f", snapshot[0].Open)
	}
	
	// Last candle should have Open=60 (the last one appended)
	if snapshot[len(snapshot)-1].Open != 60 {
		t.Errorf("Expected last candle Open=60, got %f", snapshot[len(snapshot)-1].Open)
	}
}

// TestSnapshotReturnsCopy verifies that modifying the returned slice doesn't affect the buffer
func TestSnapshotReturnsCopy(t *testing.T) {
	buf := New(60)
	
	// Append a candle
	candle := schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1m",
		Open:      100.0,
		High:      101.0,
		Low:       99.0,
		Close:     100.5,
		Volume:    1000.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	}
	buf.Append(candle)
	
	// Get snapshot and modify it
	snapshot := buf.Snapshot()
	if len(snapshot) == 0 {
		t.Fatal("Snapshot is empty")
	}
	snapshot[0].Open = 999.0
	
	// Get snapshot again - should not be modified
	snapshot2 := buf.Snapshot()
	if len(snapshot2) == 0 {
		t.Fatal("Second snapshot is empty")
	}
	
	if snapshot2[0].Open != 100.0 {
		t.Errorf("Expected buffer unchanged (Open=100.0), got %f", snapshot2[0].Open)
	}
}

// TestConcurrentReadWrite verifies thread safety with concurrent operations
func TestConcurrentReadWrite(t *testing.T) {
	buf := New(60)
	done := make(chan bool)
	
	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			candle := schemas.Candle{
				Symbol:    "BTCUSDT",
				Interval:  "1m",
				Open:      float64(i),
				High:      float64(i) + 1,
				Low:       float64(i) - 1,
				Close:     float64(i) + 0.5,
				Volume:    float64(i) * 100,
				CloseTime: time.Now(),
				Timestamp: time.Now(),
			}
			buf.Append(candle)
		}
		done <- true
	}()
	
	// Reader goroutine 1
	go func() {
		for i := 0; i < 100; i++ {
			_ = buf.Snapshot()
			_ = buf.Len()
			_ = buf.IsFull()
		}
		done <- true
	}()
	
	// Reader goroutine 2
	go func() {
		for i := 0; i < 100; i++ {
			_ = buf.Snapshot()
			_ = buf.Len()
			_ = buf.IsFull()
		}
		done <- true
	}()
	
	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}
	
	// Final verification
	if buf.Len() > 60 {
		t.Errorf("Buffer exceeded capacity: Len() = %d", buf.Len())
	}
}

// TestNewWithInvalidSize verifies that New rejects invalid sizes
func TestNewWithInvalidSize(t *testing.T) {
	// Test zero capacity
	buf := New(0)
	if buf != nil {
		t.Error("Expected nil buffer for zero capacity")
	}
	
	// Test negative capacity
	buf = New(-1)
	if buf != nil {
		t.Error("Expected nil buffer for negative capacity")
	}
}

// TestEmptyBuffer verifies behavior with empty buffer
func TestEmptyBuffer(t *testing.T) {
	buf := New(60)
	
	if buf.Len() != 0 {
		t.Errorf("Expected Len() = 0 for empty buffer, got %d", buf.Len())
	}
	
	if buf.IsFull() {
		t.Error("Expected IsFull() = false for empty buffer")
	}
	
	snapshot := buf.Snapshot()
	if len(snapshot) != 0 {
		t.Errorf("Expected empty snapshot, got %d elements", len(snapshot))
	}
}

// TestSnapshotOrder verifies candles are returned in chronological order
func TestSnapshotOrder(t *testing.T) {
	buf := New(60)
	
	// Append candles with known order
	for i := 0; i < 5; i++ {
		candle := schemas.Candle{
			Symbol:    "BTCUSDT",
			Interval:  "1m",
			Open:      float64(i * 10),
			High:      float64(i*10) + 1,
			Low:       float64(i*10) - 1,
			Close:     float64(i*10) + 0.5,
			Volume:    float64(i) * 100,
			CloseTime: time.Now(),
			Timestamp: time.Now(),
		}
		buf.Append(candle)
	}
	
	snapshot := buf.Snapshot()
	if len(snapshot) != 5 {
		t.Fatalf("Expected 5 candles, got %d", len(snapshot))
	}
	
	// Verify order is maintained
	for i, candle := range snapshot {
		expectedOpen := float64(i * 10)
		if candle.Open != expectedOpen {
			t.Errorf("Position %d: expected Open=%f, got %f", i, expectedOpen, candle.Open)
		}
	}
}
