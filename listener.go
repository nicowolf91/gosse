package gosse

import (
	"context"
	"fmt"
	"github.com/imkira/go-observer"
	"net/http"
	"time"
)

var ErrStreamingNotSupported = fmt.Errorf("streaming not supported")

type listenerConn struct {
	responseWriteFlusher
	keepAliveInterval time.Duration
	keepAliveMsgBytes []byte
}

func newListenerConn(
	w http.ResponseWriter,
	keepAliveInterval time.Duration,
	keepAliveMsgBytes []byte,
) (*listenerConn, error) {
	rwf, ok := w.(responseWriteFlusher)
	if !ok {
		return nil, ErrStreamingNotSupported
	}

	rwf.Header().Set("Content-Type", "text/event-stream")

	return &listenerConn{
		responseWriteFlusher: rwf,
		keepAliveInterval:    keepAliveInterval,
		keepAliveMsgBytes:    keepAliveMsgBytes,
	}, nil
}

func (l *listenerConn) run(
	ctx context.Context,
	replay [][]byte,
	stream observer.Stream,
) {
	for i := 0; i < len(replay); i++ {
		_ = l.send(replay[i])
	}

	timer := time.NewTimer(l.keepAliveInterval)
	defer timer.Stop()

	for {
		if l.keepAliveInterval > 0 {
			timer.Reset(l.keepAliveInterval)
		}

		select {
		case <-stream.Changes():
			next := stream.Next()
			bytesToSend, ok := next.([]byte)
			if ok {
				err := l.send(bytesToSend)
				if err != nil {
					return
				}
			}

		case <-timer.C:
			_ = l.send(l.keepAliveMsgBytes)

		case <-ctx.Done():
			return
		}
	}
}

func (l *listenerConn) send(b []byte) error {
	_, err := l.Write(b)
	if err != nil {
		return err
	}
	l.Flush()
	return nil
}

type responseWriteFlusher interface {
	http.ResponseWriter
	http.Flusher
}

var DefaultKeepAliveMessageBytes = DefaultMessageToBytesConverter(NewMessage().WithData([]byte(": ")))

const headerLastEventID = "Last-Event-ID"
