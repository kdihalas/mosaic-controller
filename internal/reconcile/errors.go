package reconcile

import "fmt"

type ErrorClass string

const (
	Retryable ErrorClass = "Retryable"
	Stalling  ErrorClass = "Stalling"
)

type StageError struct {
	Class  ErrorClass
	Reason string
	Err    error
}

func (e *StageError) Error() string { return fmt.Sprintf("%s: %v", e.Reason, e.Err) }
func (e *StageError) Unwrap() error { return e.Err }

func Retry(reason string, err error) error {
	return &StageError{Class: Retryable, Reason: reason, Err: err}
}
func Stall(reason string, err error) error {
	return &StageError{Class: Stalling, Reason: reason, Err: err}
}

func Classify(err error) (ErrorClass, string) {
	if e, ok := err.(*StageError); ok {
		return e.Class, e.Reason
	}
	return Retryable, "ProgressingWithRetry"
}
