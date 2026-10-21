package testutil

import (
	"sync"
	"time"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/clock"
)

// FakeClock is a Clock whose time changes only when a test moves it.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFakeClock starts at t.
func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{now: t}
}

// Now returns the time last set by Set or Advance.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set moves the clock to t.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Advance adds d to the current time.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var _ clock.Clock = (*FakeClock)(nil)
