package easyext

import (
	"fmt"
	"strings"
)

// RegistrationError reports all independent problems found while compiling an assembly.
type RegistrationError struct {
	Problems []string
}

func (e *RegistrationError) Error() string {
	if len(e.Problems) == 1 {
		return "easyext: invalid assembly: " + e.Problems[0]
	}
	return "easyext: invalid assembly:\n  - " + strings.Join(e.Problems, "\n  - ")
}

// Reason classifies a ResolutionError.
type Reason int

const (
	// NoBinding: this Registry has no resolution bound to the supplied context.
	NoBinding Reason = iota + 1
	// NoBusinessMatched: no business matched the request.
	NoBusinessMatched
	// MultipleBusinessesMatched: more than one business matched the request.
	MultipleBusinessesMatched
	// ExtensionNotFound: the extension point was not registered with Builder.Point.
	ExtensionNotFound
)

func (r Reason) String() string {
	switch r {
	case NoBinding:
		return "NO_BINDING"
	case NoBusinessMatched:
		return "NO_BUSINESS_MATCHED"
	case MultipleBusinessesMatched:
		return "MULTIPLE_BUSINESSES_MATCHED"
	case ExtensionNotFound:
		return "EXTENSION_NOT_FOUND"
	default:
		return fmt.Sprintf("Reason(%d)", int(r))
	}
}

// ResolutionError is returned when a request cannot be resolved or an extension cannot be looked up.
// Use errors.Is with Err* sentinels or errors.As to inspect Reason and Detail.
type ResolutionError struct {
	Reason Reason
	Detail string
}

func (e *ResolutionError) Error() string {
	if e.Detail == "" {
		return "easyext: " + e.Reason.String()
	}
	return "easyext: " + e.Reason.String() + ": " + e.Detail
}

func (e *ResolutionError) Is(target error) bool {
	t, ok := target.(*ResolutionError)
	return ok && t != nil && e != nil && t.Reason == e.Reason
}

// Sentinels for errors.Is.
var (
	ErrNoBinding                 = &ResolutionError{Reason: NoBinding}
	ErrNoBusinessMatched         = &ResolutionError{Reason: NoBusinessMatched}
	ErrMultipleBusinessesMatched = &ResolutionError{Reason: MultipleBusinessesMatched}
	ErrExtensionNotFound         = &ResolutionError{Reason: ExtensionNotFound}
)
