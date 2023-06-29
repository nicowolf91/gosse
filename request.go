package gosse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/imkira/go-observer/v2"
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
	baseCtx context.Context,
	writer io.Writer,
	messageToBytesConverter MessageToBytesConverter,
	stream observer.Stream[Messager],
	replayMessages []Messager,
	keepAliveInterval time.Duration,
	keepAliveMsg Messager,
) {
	for _, replayMessage := range replayMessages {
		_, err := writer.Write(messageToBytesConverter.Convert(replayMessage))
		if err != nil {
			return
		}
	}

	// send keep alive on serve to instantly trigger SSE onopen in firefox and chrome
	if isKeepAliveActive(keepAliveMsg, keepAliveInterval) {
		_, err := writer.Write(messageToBytesConverter.Convert(keepAliveMsg))
		if err != nil {
			return
		}
	}

	for {
		innerCtx, cancel := context.WithCancel(baseCtx)
		if isKeepAliveActive(keepAliveMsg, keepAliveInterval) {
			innerCtx, cancel = context.WithTimeout(baseCtx, keepAliveInterval)
		}

		nextMsg, err := stream.WaitNextCtx(innerCtx)
		cancel()
		if err != nil {
			if baseCtx.Err() != nil || !errors.Is(err, context.DeadlineExceeded) {
				return
			}

			_, keepAliveErr := writer.Write(messageToBytesConverter.Convert(keepAliveMsg))
			if keepAliveErr != nil {
				return
			}
			continue
		}

		_, err = writer.Write(messageToBytesConverter.Convert(nextMsg))
		if err != nil {
			return
		}
	}
}

func isKeepAliveActive(keepAliveMsg Messager, keepAliveInterval time.Duration) bool {
	return keepAliveMsg != nil && keepAliveInterval > 0
}
