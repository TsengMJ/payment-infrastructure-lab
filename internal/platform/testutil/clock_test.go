package testutil

import (
	"testing"
	"time"
)

func TestFakeClockAdvanceMovesNowForward(t *testing.T) {
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	c := NewFakeClock(start)
	if got := c.Now(); !got.Equal(start) {
		t.Fatalf("Now() = %s, want %s", got, start)
	}
	c.Advance(90 * time.Second)
	want := start.Add(90 * time.Second)
	if got := c.Now(); !got.Equal(want) {
		t.Fatalf("Now() = %s, want %s", got, want)
	}
}
