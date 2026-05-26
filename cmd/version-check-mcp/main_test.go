package main

import (
	"errors"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestIsCleanShutdown(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: true},
		{name: "connection closed", err: sdkmcp.ErrConnectionClosed, want: true},
		{name: "wrapped connection closed", err: errors.Join(errors.New("wrapper"), sdkmcp.ErrConnectionClosed), want: true},
		{name: "substring false positive", err: errors.New("upstream server is closing cache connection"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCleanShutdown(tt.err); got != tt.want {
				t.Fatalf("isCleanShutdown(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}
