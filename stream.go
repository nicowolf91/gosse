package gosse

import (
	"context"
	"github.com/imkira/go-observer"
)

type Stream interface {
	observer.Stream
	WaitNextCtx(ctx context.Context) (interface{}, error)
}

type stream struct {
	observer.Stream
}

func newStream(s observer.Stream) Stream {
	return &stream{Stream: s}
}

func (s *stream) WaitNextCtx(ctx context.Context) (interface{}, error) {
	select {
	case <-s.Changes():
		return s.Next(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
