// Package clock is the time source for domain code.
// Callers take a Clock instead of calling time.Now, so tests can move time.
package clock

import "time"

// Clock returns the current time.
type Clock interface {
	Now() time.Time
}

// System reads the process clock.
type System struct{}

// Now returns time.Now.
func (System) Now() time.Time { return time.Now() }
