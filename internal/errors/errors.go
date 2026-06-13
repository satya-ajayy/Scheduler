package errors

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Error defines a standard application error.
type Error struct {
	Kind       Kind   `json:"kind"`
	Message    string `json:"message"`
	WrappedErr error  `json:"wrapped_err,omitempty"`
}

func (e *Error) Error() string {
	if e.WrappedErr != nil {
		return fmt.Sprintf("%s: %s: %s", e.Kind, e.Message, e.WrappedErr)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *Error) Unwrap() error {
	return e.WrappedErr
}

// Kind defines the kind or class of an error.
type Kind uint8

// Transport agnostic error "kinds"
const (
	Other        Kind = iota // Unclassified error
	Internal                 // Internal error
	Conflict                 // Conflict when an entity already exists
	Invalid                  // Invalid input, validation error etc
	NotFound                 // Entity does not exist
	Unauthorized             // Unauthorized access
	Forbidden                // Forbidden access
)

func (k Kind) String() string {
	switch k {
	case Other:
		return "unclassified error"
	case Internal:
		return "internal error"
	case Conflict:
		return "conflict"
	case Invalid:
		return "invalid input"
	case NotFound:
		return "entity not found"
	case Unauthorized:
		return "unauthorized"
	case Forbidden:
		return "forbidden"
	default:
		return "unknown error kind"
	}
}

func (k Kind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// NewError constructs an *Error with explicit, typed parameters.
func NewError(kind Kind, message string, wrapped ...error) error {
	e := &Error{Kind: kind, Message: message}
	if len(wrapped) > 0 {
		e.WrappedErr = wrapped[0]
	}
	return e
}

var (
	As = errors.As
	Is = errors.Is
)
