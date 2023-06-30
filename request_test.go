package gosse

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/imkira/go-observer/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://localhost", &bytes.Buffer{})
	req.Header.Set("Last-Event-ID", "123")
	additionalHeader := http.Header{}
	additionalHeader.Set("Cache-Control", "no-cache")
	additionalHeader.Set("Connection", "keep-alive")
	sseRequest, err := NewRequest(recorder, req, additionalHeader)
	assert.NoError(t, err)
	assert.True(t, recorder.Flushed)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "123", sseRequest.LastEventID)
	assert.Len(t, recorder.Header(), 3)
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", recorder.Header().Get("Connection"))
}

func TestNewRequestNoFlusher(t *testing.T) {
	_, err := NewRequest(&noFlushMock{}, nil, nil)
	assert.Error(t, err)
	assert.Equal(t, ErrStreamingNotSupported, err)
}

func TestRequest_Write(t *testing.T) {
	mock := &responseWriteFlusherMock{}
	request := &Request{ResponseWriteFlusher: mock}
	toWrite := []byte("test")
	n, err := request.Write(toWrite)
	assert.NoError(t, err)
	assert.Equal(t, n, len(toWrite))
	assert.Equal(t, toWrite, mock.buf.Bytes())
	assert.Equal(t, 1, mock.flushed)
}

func TestExtractLastEventID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
	got := ExtractLastEventID(req)
	assert.Equal(t, "", got)

	req.Header.Set("Last-Event-ID", "id4711")
	got = ExtractLastEventID(req)
	assert.Equal(t, "id4711", got)
}

func TestServe(t *testing.T) {
	type testCase struct {
		name                string
		ctxTimeout          time.Duration
		expectedCtxError    error
		writer              *maxWriter
		expectedWriteCalled int
		replayMessages      []Messager
		messages            []Messager
		expectedMessages    []Messager
		keepAliveInterval   time.Duration
	}

	testCases := []testCase{
		{
			name:                "replay only",
			ctxTimeout:          0,
			expectedCtxError:    context.DeadlineExceeded,
			writer:              newMaxWriter(1337),
			expectedWriteCalled: 2,
			replayMessages: []Messager{
				NewMessage().WithData([]byte("replay")).WithID("1"),
				NewMessage().WithEvent("e").WithData([]byte("test")).WithID("2"),
			},
			messages: []Messager{
				NewMessage().WithEvent("e1").WithData([]byte("message 1")),
				NewMessage().WithEvent("e2").WithData([]byte("message 2")),
			},
			expectedMessages: []Messager{
				NewMessage().WithData([]byte("replay")).WithID("1"),
				NewMessage().WithEvent("e").WithData([]byte("test")).WithID("2"),
			},
			keepAliveInterval: 0,
		},
		{
			name:                "replay write error",
			ctxTimeout:          3 * time.Second,
			expectedCtxError:    nil,
			writer:              newMaxWriter(1),
			expectedWriteCalled: 2,
			replayMessages: []Messager{
				NewMessage().WithData([]byte("replay")).WithID("1"),
				NewMessage().WithEvent("e").WithData([]byte("test")).WithID("2"),
			},
			messages: nil,
			expectedMessages: []Messager{
				NewMessage().WithData([]byte("replay")).WithID("1"),
			},
			keepAliveInterval: 0,
		},
		{
			name:                "replay and normal messages, no keep alive",
			ctxTimeout:          3 * time.Second,
			expectedCtxError:    nil,
			writer:              newMaxWriter(4),
			expectedWriteCalled: 5,
			replayMessages: []Messager{
				NewMessage().WithData([]byte("replay")).WithID("1"),
				NewMessage().WithEvent("e").WithData([]byte("test")).WithID("2"),
			},
			messages: []Messager{
				NewMessage().WithEvent("e1").WithData([]byte("message 1")),
				NewMessage().WithEvent("e2").WithData([]byte("message 2")),
				DefaultKeepAliveMessage,
			},
			expectedMessages: []Messager{
				NewMessage().WithData([]byte("replay")).WithID("1"),
				NewMessage().WithEvent("e").WithData([]byte("test")).WithID("2"),
				NewMessage().WithEvent("e1").WithData([]byte("message 1")),
				NewMessage().WithEvent("e2").WithData([]byte("message 2")),
			},
			keepAliveInterval: 0,
		},
		{
			name:                "replay, normal messages and keep alive",
			ctxTimeout:          3 * time.Second,
			expectedCtxError:    nil,
			writer:              newMaxWriter(5),
			expectedWriteCalled: 6,
			replayMessages:      nil,
			messages: []Messager{
				NewMessage().WithEvent("e1").WithData([]byte("message 1")),
				NewMessage().WithEvent("e2").WithData([]byte("message 2")),
			},
			expectedMessages: []Messager{
				DefaultKeepAliveMessage,
				NewMessage().WithEvent("e1").WithData([]byte("message 1")),
				NewMessage().WithEvent("e2").WithData([]byte("message 2")),
				DefaultKeepAliveMessage,
				DefaultKeepAliveMessage,
			},
			keepAliveInterval: 1 * time.Millisecond,
		},
		{
			name:                "only normal messages, context timeout",
			ctxTimeout:          333 * time.Millisecond,
			expectedCtxError:    context.DeadlineExceeded,
			writer:              newMaxWriter(1337),
			expectedWriteCalled: 2,
			replayMessages:      nil,
			messages: []Messager{
				NewMessage().WithEvent("e1").WithData([]byte("message 1")),
				NewMessage().WithEvent("e2").WithData([]byte("message 2")),
			},
			expectedMessages: []Messager{
				NewMessage().WithEvent("e1").WithData([]byte("message 1")),
				NewMessage().WithEvent("e2").WithData([]byte("message 2")),
			},
			keepAliveInterval: 0,
		},
		{
			name:                "only keep alive",
			ctxTimeout:          30 * time.Millisecond,
			expectedCtxError:    context.DeadlineExceeded,
			writer:              newMaxWriter(1337),
			expectedWriteCalled: 2,
			replayMessages:      nil,
			messages:            nil,
			expectedMessages: []Messager{
				DefaultKeepAliveMessage, // initial one
				DefaultKeepAliveMessage, // sent via keep alive logic
			},
			keepAliveInterval: 20 * time.Millisecond,
		},
		{
			name:                "initial keep alive error",
			ctxTimeout:          3 * time.Second,
			expectedCtxError:    nil,
			writer:              newMaxWriter(0),
			expectedWriteCalled: 1,
			replayMessages:      nil,
			messages:            nil,
			expectedMessages:    nil,
			keepAliveInterval:   1 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prop := observer.NewProperty[Messager](nil)
			stream := prop.Observe()
			for _, msg := range tc.messages {
				prop.Update(msg)
			}

			ctx, cancel := context.WithTimeout(context.Background(), tc.ctxTimeout)
			serveRequest(ctx, tc.writer, DefaultMessageToBytesConverter, stream, tc.replayMessages, tc.keepAliveInterval, DefaultKeepAliveMessage)
			assert.ErrorIs(t, ctx.Err(), tc.expectedCtxError)
			cancel()

			assert.Equal(t, messagesToBytes(tc.expectedMessages, DefaultMessageToBytesConverter), tc.writer.Bytes())
			assert.Equal(t, tc.expectedWriteCalled, tc.writer.writeCalled)
		})
	}
}

type noFlushMock struct {
	header     http.Header
	statusCode int
}

func newNoFlushMock() *noFlushMock {
	return &noFlushMock{header: http.Header{}}
}

func (n *noFlushMock) Header() http.Header         { return n.header }
func (n *noFlushMock) Write(i []byte) (int, error) { return 0, nil }
func (n *noFlushMock) WriteHeader(statusCode int)  { n.statusCode = statusCode }

type maxWriter struct {
	writeCalled    int
	writesExecuted int
	maxWrites      int
	bytes.Buffer
}

func newMaxWriter(maxWrites int) *maxWriter {
	return &maxWriter{maxWrites: maxWrites}
}

func (m *maxWriter) Write(p []byte) (n int, err error) {
	m.writeCalled++
	if m.writesExecuted >= m.maxWrites {
		return 0, fmt.Errorf("max writes reached")
	}

	m.writesExecuted++
	return m.Buffer.Write(p)
}

type responseWriteFlusherMock struct {
	buf        bytes.Buffer
	statusCode int
	flushed    int
}

func (r *responseWriteFlusherMock) Header() http.Header {
	return nil
}

func (r *responseWriteFlusherMock) Write(b []byte) (int, error) {
	return r.buf.Write(b)
}

func (r *responseWriteFlusherMock) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *responseWriteFlusherMock) Flush() {
	r.flushed++
}

func messagesToBytes(messages []Messager, converter MessageToBytesConverter) []byte {
	var ret []byte
	for _, msg := range messages {
		ret = append(ret, converter.Convert(msg)...)
	}
	return ret
}
