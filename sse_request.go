package gosse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

var ErrStreamingNotSupported = fmt.Errorf("streaming not supported")

type responseWriteFlusher interface {
	http.ResponseWriter
	http.Flusher
}

type sseRequest struct {
	rwf         responseWriteFlusher
	lastEventID string
}

func newSseRequest(
	w http.ResponseWriter,
	r *http.Request,
	additionalHeader http.Header,
) (*sseRequest, error) {
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

	return &sseRequest{
		rwf:         rwf,
		lastEventID: extractLastEventID(r),
	}, nil
}

func (s *sseRequest) Write(b []byte) (int, error) {
	n, err := s.rwf.Write(b)
	if err != nil {
		return 0, err
	}
	s.rwf.Flush()
	return n, nil
}

const headerLastEventID = "Last-Event-ID"

func extractLastEventID(r *http.Request) string {
	return r.Header.Get(headerLastEventID)
}

func serveSseRequest(
	ctx context.Context,
	writer io.Writer,
	stream Stream,
	replay []byte,
	keepAliveInterval time.Duration,
	keepAliveMsgBytes []byte,
) {
	_, _ = writer.Write(replay)

	timer := time.NewTimer(keepAliveInterval)
	defer timer.Stop()

	for {
		if isKeepAliveActive(keepAliveInterval) {
			timer.Reset(keepAliveInterval)
		}

		select {
		case <-stream.Changes():
			next := stream.Next()
			bytesToSend, ok := next.([]byte)
			if ok {
				_, err := writer.Write(bytesToSend)
				if err != nil {
					return
				}
			}

		case <-timer.C:
			if isKeepAliveActive(keepAliveInterval) {
				_, err := writer.Write(keepAliveMsgBytes)
				if err != nil {
					return
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

func isKeepAliveActive(keepAliveInterval time.Duration) bool {
	return keepAliveInterval > 0
}
