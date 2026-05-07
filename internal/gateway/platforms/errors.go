package platforms

import "errors"

// ErrMissingToken is returned when a platform requires credentials that are not provided.
var ErrMissingToken = errors.New("platform: required token or credentials not provided")
