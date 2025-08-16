package sync

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

const DeadlockDetection = 40 * time.Second

var ErrCoroutineAlreadyFinished = errors.New("coroutine already finished")

type CoroutineCreator interface {
	NewCoroutine(ctx Context, fn func(Context) error)
}

type Coroutine interface {
	// Execute continues execution of a blocked corouting and waits until
	// it is finished or blocked again
	Execute()

	// Yield yields execution and stops coroutine execution
	Yield()

	// Exit prevents a _blocked_ Coroutine from continuing
	Exit()

	Blocked() bool
	Finished() bool
	Progress() bool

	Error() error

	SetCoroutineCreator(creator CoroutineCreator)
}

type key int

var coroutinesCtxKey key

type logger interface {
	Println(v ...any)
}

type coState struct {
	blocking   chan bool    // coroutine is going to be blocked
	unblock    chan bool    // channel to unblock block coroutine
	blocked    atomic.Value // coroutine is currently blocked
	finished   atomic.Value // coroutine finished executing
	shouldExit atomic.Value // coroutine should exit
	progress   atomic.Value // did the coroutine make progress since last yield?

	err error

	// logger logger
	// idx    int

	deadlockDetection time.Duration

	creator CoroutineCreator
}

func NewCoroutine(ctx Context, fn func(ctx Context) error) Coroutine {
	s := newState()
	ctx = withCoState(ctx, s)

	go func() {
		defer s.finish() // Ensure we always mark the coroutine as finished
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok && errors.Is(err, ErrCoroutineAlreadyFinished) {
					// Ignore this specific error
					return
				}

				s.err = fmt.Errorf("panic: %v", r)
			}
		}()

		// yield before the first execution
		s.yield(false)

		s.err = fn(ctx)
	}()

	return s
}

// var i = 0

func newState() *coState {
	// i++

	c := &coState{
		blocking: make(chan bool, 1),
		unblock:  make(chan bool),
		// Only used while debugging issues, default to discarding log messages
		// logger:            log.New(os.Stderr, fmt.Sprintf("[co %v]", i), log.Lmsgprefix|log.Ltime),
		// idx:               i,
		deadlockDetection: DeadlockDetection,
	}

	// Start out as blocked
	c.blocked.Store(true)

	return c
}

func (s *coState) finish() {
	s.finished.Store(true)
	s.blocking <- true

	// s.logger.Println("finish")
}

func (s *coState) SetCoroutineCreator(creator CoroutineCreator) {
	s.creator = creator
}

func (s *coState) Finished() bool {
	v, ok := s.finished.Load().(bool)
	return ok && v
}

func (s *coState) Blocked() bool {
	v, ok := s.blocked.Load().(bool)
	return ok && v
}

func (s *coState) MadeProgress() {
	s.progress.Store(true)
}

func (s *coState) ResetProgress() {
	s.progress.Store(false)
}

func (s *coState) Progress() bool {
	x := s.progress.Load()
	v, ok := x.(bool)
	return ok && v
}

func (s *coState) Yield() {
	s.yield(true)
}

func (s *coState) yield(markBlocking bool) {
	// s.logger.Println("yielding")

	if markBlocking {
		if s.shouldExit.Load() != nil {
			// s.logger.Println("yielding, but should exit")
			panic(ErrCoroutineAlreadyFinished)
		}

		s.blocked.Store(true)

		s.blocking <- true
	}

	// s.logger.Println("yielded")

	// Wait for the next Execute() call
	<-s.unblock

	// Once we're here, another Execute() call has been made. s.blocking is empty

	if s.shouldExit.Load() != nil {
		// s.logger.Println("exiting")

		// Goexit runs all deferred functions, which includes calling finish() in the main
		// execution function. That marks the coroutine as finished and blocking.
		runtime.Goexit()
	}

	s.blocked.Store(false)

	// s.logger.Println("done yielding, continuing")
}

func (s *coState) Execute() {
	s.ResetProgress()

	if s.Finished() {
		// s.logger.Println("execute: already finished")
		return
	}

	t := time.NewTimer(s.deadlockDetection)
	defer t.Stop()

	// s.logger.Println("execute: unblocking")
	s.unblock <- true
	// s.logger.Println("execute: unblocked")

	runtime.Gosched()

	// Run until blocked (which is also true when finished)
	select {
	case <-s.blocking:
		// s.logger.Println("execute: blocked")
	case <-t.C:
		panic("coroutine timed out")
	}
}

func (s *coState) Exit() {
	// s.logger.Println("exit")

	if s.Finished() {
		return
	}

	s.shouldExit.Store(true)
	s.Execute()
}

func (s *coState) Error() error {
	return s.err
}

func withCoState(ctx Context, s *coState) Context {
	return WithValue(ctx, coroutinesCtxKey, s)
}

func getCoState(ctx Context) *coState {
	s, ok := ctx.Value(coroutinesCtxKey).(*coState)
	if !ok {
		panic("could not find coroutine state")
	}

	return s
}
