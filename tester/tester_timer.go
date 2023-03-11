package tester

type timeMode int

const (
	// TM_TimeTravel is the default time mode. Time is advanced by directly jumping to the next timer ready to be fired.
	TM_TimeTravel timeMode = iota

	// TM_WallClock prevents time traveling. Timers are only fired when the time has actually passed.
	TM_WallClock
)

func (tm timeMode) String() string {
	switch tm {
	case TM_TimeTravel:
		return "TimeTravel"
	case TM_WallClock:
		return "WallClock"
	default:
		return "Unknown"
	}
}
