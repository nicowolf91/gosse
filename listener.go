package gosse

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

var ErrStreamingNotSupported = fmt.Errorf("streaming not supported")

type listener struct {
	rwf               responseWriteFlusher
	keepAliveInterval time.Duration
	keepAliveMsgBytes []byte
}

func newListener(
	w http.ResponseWriter,
	additionalHeader http.Header,
	keepAliveInterval time.Duration,
	keepAliveMsgBytes []byte,
) (*listener, error) {
	rwf, ok := w.(responseWriteFlusher)
	if !ok {
		return nil, ErrStreamingNotSupported
	}

	rwf.Header().Set("Content-Type", "text/event-stream")

	for key, values := range additionalHeader {
		for _, value := range values {
			rwf.Header().Add(key, value)
		}
	}

	rwf.WriteHeader(http.StatusOK)
	rwf.Flush()

	return &listener{
		rwf:               rwf,
		keepAliveInterval: keepAliveInterval,
		keepAliveMsgBytes: keepAliveMsgBytes,
	}, nil
}

func (l *listener) run(
	ctx context.Context,
	replay []byte,
	stream Stream,
) {
	_, _ = l.Write(replay)

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
				_, err := l.Write(bytesToSend)
				if err != nil {
					return
				}
			}

		case <-timer.C:
			_, _ = l.Write(l.keepAliveMsgBytes)

		case <-ctx.Done():
			return
		}
	}
}

func (l *listener) Write(b []byte) (int, error) {
	n, err := l.rwf.Write(b)
	if err != nil {
		return 0, err
	}
	l.rwf.Flush()
	return n, nil
}

type responseWriteFlusher interface {
	http.ResponseWriter
	http.Flusher
}

var DefaultKeepAliveMessage = NewMessage().WithData([]byte(": "))

const headerLastEventID = "Last-Event-ID"
