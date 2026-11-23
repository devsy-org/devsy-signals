package envelope

// Level is the record severity.
type Level string

// Levels.
const (
	LevelTrace Level = "trace"
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Valid reports whether l is a known level.
func (l Level) Valid() bool {
	return l.Severity() != 0
}

// Severity returns the OpenTelemetry severity number, or 0 for an unknown level.
func (l Level) Severity() int {
	switch l {
	case LevelTrace:
		return 1
	case LevelDebug:
		return 5
	case LevelInfo:
		return 9
	case LevelWarn:
		return 13
	case LevelError:
		return 17
	}
	return 0
}
