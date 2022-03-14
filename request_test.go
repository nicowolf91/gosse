package gosse

import (
	"bytes"
	"context"
	"fmt"
	"github.com/imkira/go-observer"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestServeRequestReplayOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	replay := []byte("data: replay\nid: 1\n\nevent: e\ndata: test\nid: 2\n\n")

	buf := &bytes.Buffer{}
	serveRequest(ctx, buf, nopStreamMock{}, replay, 0, nil)

	assert.Equal(t, replay, buf.Bytes())
}

func TestServeRequestNoKeepAlive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	replay := []byte("data: replay\nid: 1\n\nevent: e\ndata: test\nid: 2\n\n")
	messages := [][]byte{
		DefaultMessageToBytesConverter.Convert(NewMessage().WithEvent("e1").WithData([]byte("message 1"))),
		DefaultMessageToBytesConverter.Convert(NewMessage().WithEvent("e2").WithData([]byte("message 2"))),
	}

	writer := newMaxWriter(len(messages))

	expected := make([]byte, len(replay))
	copy(expected, replay)

	broker := NewBroker()
	stream := broker.Subscribe()
	for _, msg := range messages {
		broker.Publish(msg)
		expected = append(expected, msg...)
	}

	// ensure that serveRequest handles unexpected/invalid stream elements correctly
	broker.Publish(nil)
	broker.Publish(13.37)

	// trigger error in writer
	broker.Publish([]byte("boom"))

	serveRequest(ctx, writer, stream, replay, 0, nil)

	assert.Equal(t, expected, writer.Bytes())
}

func TestServeRequestWithKeepAlive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages := [][]byte{
		DefaultMessageToBytesConverter.Convert(NewMessage().WithEvent("e1").WithData([]byte("message 1"))),
		DefaultMessageToBytesConverter.Convert(NewMessage().WithEvent("e2").WithData([]byte("message 2"))),
	}

	broker := NewBroker()
	stream := broker.Subscribe()
	for _, msg := range messages {
		broker.Publish(msg)
	}

	keepAliveBytes := DefaultMessageToBytesConverter.Convert(DefaultKeepAliveMessage)

	writer := newMaxWriter(len(messages) + 1)
	serveRequest(ctx, writer, stream, nil, 1*time.Millisecond, keepAliveBytes)

	splitMinLen := len(messages) + 1 + 1 // one keep alive msg + one empty split entry at the end

	keepAliveDetected := false
	split := bytes.SplitAfter(writer.Bytes(), []byte("\n\n"))
	assert.True(t, len(split) >= splitMinLen)
	assert.Equal(t, messages[0], split[0])
	assert.Equal(t, messages[1], split[1])
	for i := len(messages); i < len(split)-1; i++ { // last split entry is always empty
		assert.Equal(t, keepAliveBytes, split[i])
		keepAliveDetected = true
	}
	assert.True(t, keepAliveDetected)
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

type nopStreamMock struct{}

func (n nopStreamMock) Value() interface{}                                   { return nil }
func (n nopStreamMock) Changes() chan struct{}                               { return make(chan struct{}) }
func (n nopStreamMock) Next() interface{}                                    { return nil }
func (n nopStreamMock) HasNext() bool                                        { return false }
func (n nopStreamMock) WaitNext() interface{}                                { return nil }
func (n nopStreamMock) Clone() observer.Stream                               { return nopStreamMock{} }
func (n nopStreamMock) WaitNextCtx(ctx context.Context) (interface{}, error) { return nil, nil }

type maxWriter struct {
	i         int
	maxWrites int
	bytes.Buffer
}

func newMaxWriter(maxWrites int) *maxWriter {
	return &maxWriter{maxWrites: maxWrites}
}

func (m *maxWriter) Write(p []byte) (n int, err error) {
	if m.i > m.maxWrites {
		return 0, fmt.Errorf("error")
	}

	m.i++
	return m.Buffer.Write(p)
}
