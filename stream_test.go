package gosse

import (
	"context"
	"testing"

	"github.com/imkira/go-observer"
	"github.com/stretchr/testify/assert"
)

func TestStream_WaitNextCtx(t *testing.T) {
	property := observer.NewProperty(nil)
	stream := newStream(property.Observe())

	ctx, cancel := context.WithCancel(context.Background())

	property.Update("test")

	next, err := stream.WaitNextCtx(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "test", next)

	cancel()
	next, err = stream.WaitNextCtx(ctx)
	assert.Error(t, err)
	assert.Equal(t, ctx.Err(), err)
	assert.Nil(t, next)
}
