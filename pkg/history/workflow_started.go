package history

type ExecutionStartedAttributes struct {
	Name string

	Version string

	Inputs []byte

	// Scheduled time.Time // TODO: Timers
}
