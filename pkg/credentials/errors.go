package credentials

import "errors"

var (
	// ErrNotFound indicates the credential was not found in this provider.
	ErrNotFound = errors.New("credential not found")

	// ErrUnsupported indicates the operation is not supported by this provider.
	ErrUnsupported = errors.New("operation not supported by this provider")

	// ErrMissingRequired indicates a required credential could not be resolved.
	ErrMissingRequired = errors.New("required credential missing")

	// ErrSessionExpired indicates a session credential has expired and refresh is needed.
	ErrSessionExpired = errors.New("session credential expired")

	// ErrRefreshFailed indicates the session refresh (re-authentication) failed.
	ErrRefreshFailed = errors.New("session refresh failed")

	// ErrNoTerminal indicates an interactive command requires a terminal but none is available.
	ErrNoTerminal = errors.New("interactive command requires a terminal (TTY)")
)
