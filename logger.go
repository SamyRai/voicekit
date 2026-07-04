package voicekit

// Logger defines a simple logging interface
type Logger interface {
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// NoOpLogger provides a no-op implementation of Logger
type NoOpLogger struct{}

func (l *NoOpLogger) Infof(format string, args ...interface{})  {}
func (l *NoOpLogger) Warnf(format string, args ...interface{})  {}
func (l *NoOpLogger) Errorf(format string, args ...interface{}) {}

// DefaultLogger returns a no-op logger
func DefaultLogger() Logger {
	return &NoOpLogger{}
}
