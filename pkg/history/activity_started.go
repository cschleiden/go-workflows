package history

type ActivityScheduledAttributes struct {
	Name string

	Version string

	// TODO: Revisit if this is the right format
	Inputs [][]byte
}
