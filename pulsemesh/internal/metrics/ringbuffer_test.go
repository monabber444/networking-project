package metrics

import (
	"math"
	"sync"
	"testing"
	"time"
)

func TestAddAndCount(t *testing.T) {
	rb := NewRingBuffer(10)
	if rb.Count() != 0 {
		t.Fatalf("expected count 0, got %d", rb.Count())
	}

	for i := 0; i < 5; i++ {
		rb.Add(Sample{RTTNanos: int64(i) * 1e6, Timestamp: time.Now()})
	}
	if rb.Count() != 5 {
		t.Fatalf("expected count 5, got %d", rb.Count())
	}
}

func TestRingBufferOverflow(t *testing.T) {
	const cap = 1000
	rb := NewRingBuffer(cap)

	for i := 0; i < 1500; i++ {
		rb.Add(Sample{RTTNanos: int64(i) * 1e6, Timestamp: time.Now()})
	}
	if rb.Count() != cap {
		t.Fatalf("expected count %d after overflow, got %d", cap, rb.Count())
	}
}

func TestPercentileKnownData(t *testing.T) {
	rb := NewRingBuffer(100)

	// Add samples 1ms through 100ms.
	for i := 1; i <= 100; i++ {
		rb.Add(Sample{
			RTTNanos:  int64(i) * int64(time.Millisecond),
			Timestamp: time.Now(),
		})
	}

	// P50 should be around 50.5ms (linear interpolation of indices 49 & 50).
	p50 := rb.P50()
	if p50 < 50*time.Millisecond || p50 > 51*time.Millisecond {
		t.Errorf("P50 = %v, want ~50.5ms", p50)
	}

	// P99 should be very close to 99.01ms–100ms.
	p99 := rb.P99()
	if p99 < 99*time.Millisecond || p99 > 100*time.Millisecond {
		t.Errorf("P99 = %v, want ~99–100ms", p99)
	}

	// P95 in range.
	p95 := rb.P95()
	if p95 < 94*time.Millisecond || p95 > 96*time.Millisecond {
		t.Errorf("P95 = %v, want ~95ms", p95)
	}
}

func TestPercentileExcludesLost(t *testing.T) {
	rb := NewRingBuffer(100)

	// Only lost packets.
	for i := 0; i < 10; i++ {
		rb.Add(Sample{RTTNanos: 1e6, Timestamp: time.Now(), Lost: true})
	}
	if p := rb.P50(); p != 0 {
		t.Errorf("P50 with all lost = %v, want 0", p)
	}

	// Add one real sample.
	rb.Add(Sample{RTTNanos: 5 * int64(time.Millisecond), Timestamp: time.Now()})
	if p := rb.P50(); p != 5*time.Millisecond {
		t.Errorf("P50 = %v, want 5ms", p)
	}
}

func TestJitter(t *testing.T) {
	rb := NewRingBuffer(100)

	// Constant RTT → jitter should converge to 0.
	for i := 0; i < 64; i++ {
		rb.Add(Sample{
			RTTNanos:  10 * int64(time.Millisecond),
			Timestamp: time.Now(),
		})
	}
	j := rb.Jitter()
	if j > 1*time.Microsecond {
		t.Errorf("jitter with constant RTT = %v, want ~0", j)
	}

	// Alternating 10ms / 20ms → |D| = 10ms, jitter converges toward 10ms.
	rb2 := NewRingBuffer(200)
	for i := 0; i < 200; i++ {
		rtt := int64(10 * time.Millisecond)
		if i%2 == 1 {
			rtt = int64(20 * time.Millisecond)
		}
		rb2.Add(Sample{RTTNanos: rtt, Timestamp: time.Now()})
	}
	j2 := rb2.Jitter()
	// With the RFC 3550 EWMA (gain = 1/16), the jitter should converge
	// toward 10ms. Allow a generous range.
	if j2 < 5*time.Millisecond || j2 > 11*time.Millisecond {
		t.Errorf("jitter with alternating RTT = %v, want 5–11ms", j2)
	}
}

func TestLossRate(t *testing.T) {
	rb := NewRingBuffer(100)

	// Empty buffer.
	if lr := rb.LossRate(); lr != 0.0 {
		t.Errorf("empty loss rate = %v, want 0", lr)
	}

	// 30% loss.
	for i := 0; i < 100; i++ {
		rb.Add(Sample{
			RTTNanos:  1e6,
			Timestamp: time.Now(),
			Lost:      i%10 < 3,
		})
	}
	lr := rb.LossRate()
	if math.Abs(lr-0.30) > 0.01 {
		t.Errorf("loss rate = %v, want ~0.30", lr)
	}
}

func TestEmptyBuffer(t *testing.T) {
	rb := NewRingBuffer(10)

	if rb.Count() != 0 {
		t.Error("count should be 0")
	}
	if rb.P50() != 0 {
		t.Error("P50 should be 0 on empty")
	}
	if rb.P95() != 0 {
		t.Error("P95 should be 0 on empty")
	}
	if rb.P99() != 0 {
		t.Error("P99 should be 0 on empty")
	}
	if rb.Jitter() != 0 {
		t.Error("Jitter should be 0 on empty")
	}
	if rb.LossRate() != 0 {
		t.Error("LossRate should be 0 on empty")
	}
}

func TestConcurrentAccess(t *testing.T) {
	rb := NewRingBuffer(1000)

	var wg sync.WaitGroup
	const goroutines = 20
	const samplesPerGoroutine = 500

	// Writer goroutines.
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < samplesPerGoroutine; i++ {
				rb.Add(Sample{
					RTTNanos:  int64(id*1000+i) * int64(time.Microsecond),
					Timestamp: time.Now(),
					Lost:      i%50 == 0,
				})
			}
		}(g)
	}

	// Reader goroutines hammering all methods concurrently.
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < samplesPerGoroutine; i++ {
				_ = rb.Count()
				_ = rb.P50()
				_ = rb.P95()
				_ = rb.P99()
				_ = rb.Jitter()
				_ = rb.LossRate()
			}
		}()
	}

	wg.Wait()

	if c := rb.Count(); c > 1000 {
		t.Errorf("count %d exceeds capacity 1000", c)
	}
}
