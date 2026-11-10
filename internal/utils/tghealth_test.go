package utils

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gotd/td/tgerr"
)

func TestIsTGConnectionError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{tgerr.New(400, "CONNECTION_NOT_INITED"), true},
		{fmt.Errorf("rpcDoRequest: %w", tgerr.New(400, "CONNECTION_NOT_INITED")), true},
		{fmt.Errorf("invoke: %w", tgerr.New(401, "AUTH_KEY_UNREGISTERED")), true},
		{tgerr.New(400, "MESSAGE_ID_INVALID"), false},
		{tgerr.New(420, "FLOOD_WAIT_30"), false},
		{context.DeadlineExceeded, false},
		{errors.New("CONNECTION_NOT_INITED"), false},
	}
	for _, c := range cases {
		if got := IsTGConnectionError(c.err); got != c.want {
			t.Errorf("IsTGConnectionError(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
