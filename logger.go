package voicekit

// Logger defines a simple logging interface
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// NoOpLogger provides a no-op implementation of Logger
type NoOpLogger struct{}

func (l *NoOpLogger) Infof(format string, args ...any)  {}
func (l *NoOpLogger) Warnf(format string, args ...any)  {}
func (l *NoOpLogger) Errorf(format string, args ...any) {}

// DefaultLogger returns a no-op logger
func DefaultLogger() Logger {
	return &NoOpLogger{}
}
