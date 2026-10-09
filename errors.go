package easyext

import (
	"fmt"
	"strings"
)

// RegistrationError reports everything wrong with an assembly. [Builder.Build] collects all problems
// instead of stopping at the first one.
type RegistrationError struct {
	Problems []string
}

func (e *RegistrationError) Error() string {
	if len(e.Problems) == 1 {
		return "easyext: invalid assembly: " + e.Problems[0]
	}
	return "easyext: invalid assembly:\n  - " + strings.Join(e.Problems, "\n  - ")
}

// Reason classifies a [ResolutionError].
type Reason int

const (
	// NoBinding: the context carries no [Resolution]; bind one with [Context.Bind] or [WithResolution].
	NoBinding Reason = iota + 1
	// NoBusinessMatched: strict mode and no business matched the param.
	NoBusinessMatched
	// MultipleBusinessesMatched: strict mode and more than one business matched the param.
	MultipleBusinessesMatched
	// BusinessNotFound: the business resolver returned the code of a business that is not registered.
	BusinessNotFound
	// ExtensionNotFound: the extension point was never registered with [Builder.Point].
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
	case BusinessNotFound:
		return "BUSINESS_NOT_FOUND"
	case ExtensionNotFound:
		return "EXTENSION_NOT_FOUND"
	default:
		return fmt.Sprintf("Reason(%d)", int(r))
	}
}

// ResolutionError is returned when a request cannot be resolved, or an extension point cannot be looked up.
// Match it with errors.Is against the Err* sentinels, or read Reason after errors.As / errors.AsType.
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

// Is reports whether target is a ResolutionError with the same Reason, so that
// errors.Is(err, ErrNoBusinessMatched) works whatever the detail.
func (e *ResolutionError) Is(target error) bool {
	t, ok := target.(*ResolutionError)
	return ok && t.Reason == e.Reason
}

// Sentinels for errors.Is.
var (
	ErrNoBinding                 = &ResolutionError{Reason: NoBinding}
	ErrNoBusinessMatched         = &ResolutionError{Reason: NoBusinessMatched}
	ErrMultipleBusinessesMatched = &ResolutionError{Reason: MultipleBusinessesMatched}
	ErrBusinessNotFound          = &ResolutionError{Reason: BusinessNotFound}
	ErrExtensionNotFound         = &ResolutionError{Reason: ExtensionNotFound}
)
