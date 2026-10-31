// Package metrics provides thread-safe metric collection primitives for PulseMesh.
package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

// Sample represents a single probe measurement.
type Sample struct {
	RTTNanos  int64     // Round-trip time in nanoseconds; meaningless if Lost is true.
	Timestamp time.Time // When the sample was recorded.
	Lost      bool      // Whether the probe packet was lost (timed out).
}

// RingBuffer is a thread-safe circular buffer that stores probe samples for a
// single peer. It supports computing percentile RTTs, jitter (RFC 3550), and
// packet loss rate.
type RingBuffer struct {
	mu       sync.Mutex
	samples  []Sample
	capacity int
	head     int // next write position
	count    int // number of valid samples (≤ capacity)

	// RFC 3550 interarrival jitter state, kept as a running average so it
	// does not require re-scanning the entire buffer.
	jitterNanos float64
	lastSample  *Sample // previous non-lost sample for jitter computation
}

// NewRingBuffer creates a RingBuffer with the given capacity.
// Capacity must be > 0; it is clamped to 1 if <= 0.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &RingBuffer{
		samples:  make([]Sample, capacity),
		capacity: capacity,
	}
}

// Add appends a sample to the ring buffer, overwriting the oldest entry when
// the buffer is full. It also updates the running RFC 3550 jitter estimate.
func (rb *RingBuffer) Add(s Sample) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.samples[rb.head] = s
	rb.head = (rb.head + 1) % rb.capacity
	if rb.count < rb.capacity {
		rb.count++
	}

	// Update RFC 3550 jitter for non-lost samples.
	if !s.Lost {
		if rb.lastSample != nil {
			// D(i,j) = difference of relative transit times.
			// Since we are measuring RTT from the same host, the "transit time"
			// proxy is just the RTT itself; the interarrival difference is:
			//   D(i,j) = (RTT_j - RTT_i) - (send_j - send_i)
			// However, for self-generated probes both send and receive clocks
			// are the same, so send-time deltas cancel and we approximate:
			//   D(i,j) = RTT_j - RTT_i
			// RFC 3550 §A.8: J(i) = J(i-1) + (|D(i-1,i)| - J(i-1)) / 16
			d := math.Abs(float64(s.RTTNanos - rb.lastSample.RTTNanos))
			rb.jitterNanos += (d - rb.jitterNanos) / 16.0
		}
		// Keep a copy so the pointer does not alias the ring slot.
		cpy := s
		rb.lastSample = &cpy
	}
}

// Count returns the number of samples currently stored.
func (rb *RingBuffer) Count() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

// P50 returns the 50th-percentile RTT. Lost packets are excluded.
// Returns 0 if no successful samples exist.
func (rb *RingBuffer) P50() time.Duration {
	return rb.percentile(0.50)
}

// P95 returns the 95th-percentile RTT.
func (rb *RingBuffer) P95() time.Duration {
	return rb.percentile(0.95)
}

// P99 returns the 99th-percentile RTT.
func (rb *RingBuffer) P99() time.Duration {
	return rb.percentile(0.99)
}

// percentile computes the p-th percentile (0 ≤ p ≤ 1) of RTT values using
// linear interpolation. Lost packets are excluded from the calculation.
func (rb *RingBuffer) percentile(p float64) time.Duration {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rtts := rb.successRTTs()
	if len(rtts) == 0 {
		return 0
	}
	sort.Slice(rtts, func(i, j int) bool { return rtts[i] < rtts[j] })

	n := float64(len(rtts))
	// Use the "C = 1" interpolation variant (same as NumPy "linear").
	rank := p * (n - 1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi || hi >= len(rtts) {
		return time.Duration(rtts[lo])
	}
	frac := rank - float64(lo)
	v := float64(rtts[lo])*(1-frac) + float64(rtts[hi])*frac
	return time.Duration(int64(math.Round(v)))
}

// Jitter returns the RFC 3550 interarrival jitter estimate.
func (rb *RingBuffer) Jitter() time.Duration {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return time.Duration(int64(math.Round(rb.jitterNanos)))
}

// LossRate returns the fraction of samples marked as lost (0.0 – 1.0).
// Returns 0 for an empty buffer.
func (rb *RingBuffer) LossRate() float64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return 0
	}
	lost := 0
	for _, s := range rb.validSamples() {
		if s.Lost {
			lost++
		}
	}
	return float64(lost) / float64(rb.count)
}

// successRTTs returns RTT nanos for all non-lost samples. Caller must hold mu.
func (rb *RingBuffer) successRTTs() []int64 {
	out := make([]int64, 0, rb.count)
	for _, s := range rb.validSamples() {
		if !s.Lost {
			out = append(out, s.RTTNanos)
		}
	}
	return out
}

// validSamples returns the slice of valid samples in insertion order.
// Caller must hold mu.
func (rb *RingBuffer) validSamples() []Sample {
	if rb.count < rb.capacity {
		return rb.samples[:rb.count]
	}
	// Full buffer: the oldest entry is at rb.head.
	out := make([]Sample, 0, rb.capacity)
	out = append(out, rb.samples[rb.head:]...)
	out = append(out, rb.samples[:rb.head]...)
	return out
}
