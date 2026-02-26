package monitor

import (
	"context"
	"errors"
	"strings"

	"github.com/docker/docker/errdefs"
)

// isContextCanceledError reports whether an error indicates context cancellation.
//
// Parameters:
//   - err: input error to classify.
//
// Returns:
//   - bool: true when error is context.Canceled/deadline exceeded or equivalent
//     text message.
func isContextCanceledError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context canceled") || strings.Contains(msg, "context deadline exceeded")
}

// isContainerNotFoundError reports whether an error indicates container
// non-existence.
//
// Parameters:
//   - err: input error to classify.
//
// Returns:
//   - bool: true when Docker not-found classification or "not found" text
//     matches.
func isContainerNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if errdefs.IsNotFound(err) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found")
}
