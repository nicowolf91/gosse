package gosse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

var ErrStreamingNotSupported = fmt.Errorf("streaming not supported")

type ResponseWriteFlusher interface {
	http.ResponseWriter
	http.Flusher
}

type Request struct {
	ResponseWriteFlusher
	LastEventID string
}

func NewRequest(
	w http.ResponseWriter,
	r *http.Request,
	additionalHeader http.Header,
) (*Request, error) {
	rwf, ok := w.(ResponseWriteFlusher)
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

	return &Request{
		ResponseWriteFlusher: rwf,
		LastEventID:          ExtractLastEventID(r),
	}, nil
}

func (r *Request) Write(b []byte) (int, error) {
	defer r.Flush()
	return r.ResponseWriteFlusher.Write(b)
}

const HeaderKeyLastEventID = "Last-Event-ID"

func ExtractLastEventID(r *http.Request) string {
	return r.Header.Get(HeaderKeyLastEventID)
}

func serveRequest(
	ctx context.Context,
	writer io.Writer,
	stream Stream,
	replay []byte,
	keepAliveInterval time.Duration,
	keepAliveMsgBytes []byte,
) {
	_, _ = writer.Write(replay)
	// send keep alive on serve to instantly trigger SSE onopen in firefox and chrome
	_, _ = writer.Write(keepAliveMsgBytes)

	timer := time.NewTimer(keepAliveInterval)
	defer timer.Stop()

	for {
		if isKeepAliveActive(keepAliveInterval) {
			timer.Reset(keepAliveInterval)
		}

		select {
		case <-stream.Changes():
			next := stream.Next()
			if next == nil {
				continue
			}

			bytesToSend, ok := next.([]byte)
			if !ok {
				continue
			}

			_, err := writer.Write(bytesToSend)
			if err != nil {
				return
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
