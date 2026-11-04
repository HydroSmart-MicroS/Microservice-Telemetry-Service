package valueobjects

import "errors"

var ErrInvalid = errors.New("invalid_reading")
var ErrConflict = errors.New("conflicting_duplicate")
var ErrNotFound = errors.New("resource_not_found")
var ErrUnauthorized = errors.New("unauthorized")
var ErrUnavailable = errors.New("dependency_unavailable")
var ErrForbidden = errors.New("forbidden")
