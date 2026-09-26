package daemon

import (
	"sync"
	"time"
)

// [DEBOUNCER]

// Debouncer coalesces bursts of triggers into a single run.
// After Trigger, fn runs once the interval elapses with no further triggers.
// If fn is still running when a trigger arrives, a re-run is queued instead of
// overlapping.
type Debouncer struct {
	mu       sync.Mutex
	interval time.Duration
	fn       func()

	timer   *time.Timer
	running bool
	pending bool
	stopped bool
	wg      sync.WaitGroup
}

// NewDebouncer creates a Debouncer that invokes fn after each quiet interval.
func NewDebouncer(interval time.Duration, fn func()) *Debouncer {
	return &Debouncer{interval: interval, fn: fn}
}

// Trigger (re)schedules fn to run after the quiet interval.
func (d *Debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	if d.running {
		d.pending = true
		return
	}

	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.interval, d.fire)
}

func (d *Debouncer) fire() {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	if d.running {
		d.pending = true
		d.mu.Unlock()
		return
	}
	d.running = true
	d.wg.Add(1)
	d.mu.Unlock()

	d.fn()

	d.mu.Lock()
	d.running = false
	d.wg.Done()
	if d.pending && !d.stopped {
		d.pending = false
		d.timer = time.AfterFunc(d.interval, d.fire)
	}
	d.mu.Unlock()
}

// Stop cancels pending runs. It does not interrupt an in-flight fn.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true
	d.pending = false
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

// Wait blocks until any in-flight run completes. Call after Stop.
func (d *Debouncer) Wait() {
	d.wg.Wait()
}
