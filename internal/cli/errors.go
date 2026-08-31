// Package cli provides shared helpers for the centmem command-line interface:
// error output shapes, exit codes, and JSON writing conventions.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Exit codes (stable contract, see docs/cli-contract.md).
const (
	ExitOK       = 0
	ExitError    = 1
	ExitNotFound = 2
	ExitConflict = 3
)

// ErrOut is the JSON error shape written to stderr.
type ErrOut struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// Errorf builds an ErrOut and wraps it with an error value carrying the exit code.
type codedError struct {
	code int
	out  ErrOut
}

func (e *codedError) Error() string { return e.out.Message }

// ExitCode returns the process exit code for this error.
func (e *codedError) ExitCode() int { return e.code }

// E builds a coded error. Used by handlers to signal a specific exit code.
func E(code int, codeName, message string, hint string) error {
	return &codedError{code: code, out: ErrOut{Code: codeName, Message: message, Hint: hint}}
}

// NotFoundf builds a not-found (exit 2) error.
func NotFoundf(format string, args ...any) error {
	return E(ExitNotFound, "NOT_FOUND", fmt.Sprintf(format, args...), "")
}

// Conflictf builds a conflict (exit 3) error.
func Conflictf(format string, args ...any) error {
	return E(ExitConflict, "CONFLICT", fmt.Sprintf(format, args...), "")
}

// Invalidf builds an invalid-input (exit 1) error.
func Invalidf(format string, args ...any) error {
	return E(ExitError, "INVALID", fmt.Sprintf(format, args...), "")
}

// Internalf builds an internal (exit 1) error.
func Internalf(format string, args ...any) error {
	return E(ExitError, "INTERNAL", fmt.Sprintf(format, args...), "")
}

// ExitCodeFor returns the process exit code implied by err, defaulting to 1.
func ExitCodeFor(err error) int {
	if c, ok := err.(*codedError); ok {
		return c.ExitCode()
	}
	return ExitError
}

// WriteError writes err as a JSON error object to w (typically stderr).
func WriteError(w io.Writer, err error) {
	out := ErrOut{Code: "INTERNAL", Message: err.Error()}
	if c, ok := err.(*codedError); ok {
		out = c.out
	}
	enc := json.NewEncoder(w)
	_ = enc.Encode(map[string]ErrOut{"error": out})
}

// WriteJSON writes v to w as compact JSON.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	return enc.Encode(v)
}

// PrintJSON writes v to stdout as compact JSON.
func PrintJSON(v any) error {
	return WriteJSON(os.Stdout, v)
}
