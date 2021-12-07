package history

type ExecutionStartedAttributes struct {
	Name string

	Version string

	// TODO: Revisit if this is the right format
	Inputs [][]byte

	// Scheduled time.Time // TODO: Timers
}
