package sync

type Scheduler struct {
	coroutines []Coroutine
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		coroutines: make([]Coroutine, 0),
	}
}

// Starts a new co-routine and tracks it in this scheduler
func (s *Scheduler) NewCoroutine(ctx Context, fn func(Context) error) {
	c := NewCoroutine(ctx, fn)
	s.coroutines = append(s.coroutines, c)
	c.SetCoroutineCreator(s)
}

// Execute executes all coroutines until they are all blocked
func (s *Scheduler) Execute() error {
	allBlocked := false
	for !allBlocked {
		allBlocked = true
		for i := 0; i < len(s.coroutines); i++ {
			c := s.coroutines[i]

			c.Execute()

			if c.Finished() {
				// Coroutine finished, this counts as progress
				allBlocked = false

				// remove from list
				s.coroutines[i] = nil
				s.coroutines = append(s.coroutines[:i], s.coroutines[i+1:]...)
				i--

				if err := c.Error(); err != nil {
					// Coroutine encountered an error, abort execution
					return err
				}
			} else {
				// Determine if coroutine made any progress or if it stayed blocked
				allBlocked = allBlocked && !c.Progress()
			}
		}
	}

	return nil
}

func (s *Scheduler) RunningCoroutines() int {
	return len(s.coroutines)
}

func (s *Scheduler) Exit() {
	for _, c := range s.coroutines {
		c.Exit()
	}
}
