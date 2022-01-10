package sync

import (
	"fmt"
	"io"
	"log"
	"runtime"
	"sync/atomic"
	"time"
)

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
}

type key int

var coroutinesCtxKey key

type logger interface {
	Println(v ...interface{})
}

type coState struct {
	blocking   chan bool    // coroutine is going to be blocked
	unblock    chan bool    // channel to unblock block coroutine
	blocked    atomic.Value // coroutine is currently blocked
	finished   atomic.Value // coroutine finished executing
	shouldExit atomic.Value // coroutine should exit
	progress   atomic.Value // did the coroutine make progress since last yield?

	err error

	logger logger

	deadlockDetection time.Duration
}

func NewCoroutine(ctx Context, fn func(ctx Context)) Coroutine {
	s := newState()
	ctx = withCoState(ctx, s)

	go func() {
		defer s.finish() // Ensure we always mark the coroutine as finished
		defer func() {
			if r := recover(); r != nil {
				s.err = fmt.Errorf("panic: %v", r)
			}
		}()

		// yield before the first execution
		s.yield(false)

		fn(ctx)
	}()

	return s
}

var i = 0

func newState() *coState {
	i++
	return &coState{
		blocking: make(chan bool, 1),
		unblock:  make(chan bool),
		logger:   log.New(io.Discard, "[co]", log.LstdFlags),
		//logger: log.New(os.Stderr, fmt.Sprintf("[co %v]", i), log.Lmsgprefix|log.Ltime),
		deadlockDetection: 2 * time.Second,
	}
}

func (s *coState) finish() {
	s.finished.Store(true)
	s.blocking <- true
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
	s.logger.Println("yielding")

	s.blocked.Store(true)

	if markBlocking {
		s.blocking <- true
	}

	s.logger.Println("yielded")

	<-s.unblock
	if s.shouldExit.Load() != nil {
		runtime.Goexit()
		s.logger.Println("exit")
	}

	s.blocked.Store(false)

	s.logger.Println("done yielding, continuing")
}

func (s *coState) Execute() {
	s.ResetProgress()

	if s.Finished() {
		s.logger.Println("execute: already finished")
		return
	}

	t := time.NewTimer(s.deadlockDetection)
	defer t.Stop()

	s.logger.Println("execute: unblocking")
	s.unblock <- true
	s.logger.Println("execute: unblocked")

	// Run until blocked (which is also true when finished)
	select {
	case <-s.blocking:
		s.logger.Println("execute: blocked")
	case <-t.C:
		panic("coroutine timed out")
	}
}

func (s *coState) Exit() {
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
