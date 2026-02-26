package monitor

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/docker/docker/errdefs"
)

// TestIsContextCanceledError validates context-cancel error classification.
//
// Parameters:
//   - t: testing context.
//
// Returns:
//   - None.
func TestIsContextCanceledError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "context canceled",
			err:  context.Canceled,
			want: true,
		},
		{
			name: "wrapped context canceled",
			err:  fmt.Errorf("request failed: %w", context.Canceled),
			want: true,
		},
		{
			name: "string contains context canceled",
			err:  errors.New("Get docker.sock failed: context canceled"),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("network unreachable"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isContextCanceledError(tt.err); got != tt.want {
				t.Fatalf("isContextCanceledError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsContainerNotFoundError validates container-not-found classification.
//
// Parameters:
//   - t: testing context.
//
// Returns:
//   - None.
func TestIsContainerNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "docker not found type",
			err:  errdefs.NotFound(errors.New("no such container")),
			want: true,
		},
		{
			name: "string contains not found",
			err:  errors.New("container abc not found"),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("permission denied"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isContainerNotFoundError(tt.err); got != tt.want {
				t.Fatalf("isContainerNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}
