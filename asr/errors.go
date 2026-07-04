package asr

import "fmt"

// Error wraps ASR operation failures without importing the root package.
type Error struct {
	Op        string
	SessionID string
	Err       error
}

func (e *Error) Error() string {
	if e.SessionID != "" {
		return fmt.Sprintf("ASR %s failed for session %s: %v", e.Op, e.SessionID, e.Err)
	}
	return fmt.Sprintf("ASR %s failed: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func newError(op string, err error) *Error {
	return &Error{Op: op, Err: err}
}

func newSessionError(op, sessionID string, err error) *Error {
	return &Error{Op: op, SessionID: sessionID, Err: err}
}
