package publishing

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
)

var errClaimPaused = errors.New("claim paused")

var ErrExecutorUnavailable = errors.New("发布执行器不可用")

type ExecutorStatus struct {
	State  string `json:"state"`
	Error  string `json:"error"`
	TaskID int64  `json:"taskId,omitempty"`
}

// Executor owns the worker lifecycle; task and release state remain durable.
type Executor struct {
	Worker                    *Worker
	Check                     func() error
	Enabled                   bool
	Interval                  time.Duration
	mu                        sync.Mutex
	status                    ExecutorStatus
	paused, stopping, started bool
	cancel                    context.CancelFunc
	done                      chan struct{}
	wake                      chan struct{}
}

func NewExecutor(worker *Worker, enabled bool, check func() error) *Executor {
	state := "starting"
	if !enabled {
		state = "disabled"
	}
	return &Executor{Worker: worker, Enabled: enabled, Check: check, status: ExecutorStatus{State: state}, done: make(chan struct{}), wake: make(chan struct{}, 1), Interval: time.Second}
}
func (e *Executor) Status() ExecutorStatus { e.mu.Lock(); defer e.mu.Unlock(); return e.status }
func (e *Executor) set(state string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.status.State = state
	if err != nil {
		e.status.Error = err.Error()
	} else if state == "running" {
		e.status.Error = ""
	}
}
func (e *Executor) notify() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}
func (e *Executor) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.Enabled || e.stopping {
		return ErrConflict
	}
	e.paused = true
	if e.status.State != "paused" {
		e.status.State = "pausing"
	}
	e.notify()
	return nil
}
func (e *Executor) Resume() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.Enabled || e.stopping {
		return ErrConflict
	}
	if e.status.State == "pausing" {
		return ErrConflict
	}
	e.paused = false
	e.status.State = "starting"
	e.notify()
	return nil
}
func (e *Executor) Admission() error {
	// A disabled local executor must not prevent an independently hosted worker.
	if !e.Enabled {
		return nil
	}
	if e.Check != nil {
		if err := e.Check(); err != nil {
			return errors.Join(ErrExecutorUnavailable, err)
		}
	}
	status := e.Status()
	if status.State == "blocked" || status.State == "stopping" || status.State == "stopped" {
		return errors.Join(ErrExecutorUnavailable, errors.New(status.Error))
	}
	return nil
}
func (e *Executor) Start() {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return
	}
	e.started = true
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.mu.Unlock()
	if e.Enabled {
		e.Worker.Claim = func(fn func() error) error {
			e.mu.Lock()
			if e.paused || e.stopping {
				e.mu.Unlock()
				return errClaimPaused
			}
			// Reserve this attempt before pause can be accepted. Keep the
			// lifecycle mutex free during DB work so shutdown can cancel it.
			e.mu.Unlock()
			return fn()
		}
		e.Worker.OnTask = func(id int64) { e.mu.Lock(); e.status.TaskID = id; e.mu.Unlock() }
	}
	go e.run(ctx)
}
func (e *Executor) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	if e.status.State == "stopped" {
		e.mu.Unlock()
		return nil
	}
	e.stopping = true
	e.status.State = "stopping"
	started := e.started
	cancel := e.cancel
	e.mu.Unlock()
	e.notify()
	if !started {
		e.set("stopped", nil)
		return nil
	}
	select {
	case <-e.done:
		return nil
	case <-ctx.Done():
		cancel()
		<-e.done
		return ctx.Err()
	}
}
func (e *Executor) wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-e.wake:
		return true
	case <-timer.C:
		return true
	}
}
func (e *Executor) run(ctx context.Context) {
	defer close(e.done)
	if !e.Enabled {
		return
	}
	defer e.set("stopped", nil)
	var lock *deployment.Lock
	defer func() {
		if lock != nil {
			lock.Close()
		}
	}()
	delay := e.Interval
	if delay <= 0 {
		delay = time.Second
	}
	backoff := delay
	release := func() {
		if lock != nil {
			lock.Close()
			lock = nil
		}
	}
	for {
		e.mu.Lock()
		paused, stopping, state := e.paused, e.stopping, e.status.State
		e.mu.Unlock()
		if stopping || ctx.Err() != nil {
			return
		}
		if paused {
			release()
			e.set("paused", nil)
			if !e.wait(ctx, delay) {
				return
			}
			continue
		}
		if state == "blocked" {
			release()
			if !e.wait(ctx, delay) {
				return
			}
			continue
		}
		if e.Check != nil {
			if err := e.Check(); err != nil {
				release()
				e.set("unavailable", err)
				if !e.wait(ctx, delay) {
					return
				}
				continue
			}
		}
		if lock == nil {
			acquired, err := deployment.Acquire(e.Worker.Root)
			if err != nil {
				e.set("lock_occupied", err)
				if !e.wait(ctx, backoff) {
					return
				}
				backoff = min(backoff*2, 30*time.Second)
				continue
			}
			lock = acquired
			if err = e.Worker.Recover(ctx); err != nil {
				release()
				if errors.Is(err, ErrBlocked) {
					e.set("blocked", err)
				} else {
					e.set("retrying", err)
				}
				if !e.wait(ctx, backoff) {
					return
				}
				backoff = min(backoff*2, 30*time.Second)
				continue
			}
		}
		// Serialize claiming against pause/shutdown requests. Neither waits for a build.
		e.mu.Lock()
		if e.paused || e.stopping {
			e.mu.Unlock()
			continue
		}
		e.status.State = "running"
		e.status.Error = ""
		e.mu.Unlock()
		worked, err := e.Worker.RunNext(ctx)
		e.mu.Lock()
		e.status.TaskID = 0
		e.mu.Unlock()
		if err != nil {
			release()
			if errors.Is(err, ErrBlocked) {
				e.set("blocked", err)
			} else {
				e.set("retrying", err)
			}
			if !e.wait(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = delay
		if !worked {
			if !e.wait(ctx, delay) {
				return
			}
		}
	}
}
