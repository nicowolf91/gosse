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

func TestServeRequestReplayOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	replayMessages := []Messager{
		NewMessage().WithData([]byte("replay")).WithID("1"),
		NewMessage().WithEvent("e").WithData([]byte("test")).WithID("2"),
	}

	buf := &bytes.Buffer{}
	prop := observer.NewProperty[Messager](nil)

	serveRequest(ctx, buf, DefaultMessageToBytesConverter, prop.Observe(), replayMessages, 0, nil)

	assert.Equal(t, messagesToBytes(replayMessages, DefaultMessageToBytesConverter), buf.Bytes())
}

func TestServeRequestNoKeepAlive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10000*time.Second)
	defer cancel()

	replayMessages := []Messager{
		NewMessage().WithData([]byte("replay")).WithID("1"),
		NewMessage().WithEvent("e").WithData([]byte("test")).WithID("2"),
	}
	messages := []Messager{
		NewMessage().WithEvent("e1").WithData([]byte("message 1")),
		NewMessage().WithEvent("e2").WithData([]byte("message 2")),
	}

	writer := newMaxWriter(len(replayMessages) + len(messages))

	prop := observer.NewProperty[Messager](nil)
	stream := prop.Observe()
	for _, msg := range messages {
		prop.Update(msg)
	}

	// trigger error in writer
	prop.Update(DefaultKeepAliveMessage)

	serveRequest(ctx, writer, DefaultMessageToBytesConverter, stream, replayMessages, 0, nil)

	assert.Equal(t, messagesToBytes(append(replayMessages, messages...), DefaultMessageToBytesConverter), writer.Bytes())
}

func TestServeRequestWithKeepAlive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages := []Messager{
		NewMessage().WithEvent("e1").WithData([]byte("message 1")),
		NewMessage().WithEvent("e2").WithData([]byte("message 2")),
	}

	prop := observer.NewProperty[Messager](nil)
	stream := prop.Observe()
	for _, msg := range messages {
		prop.Update(msg)
	}

	writer := newMaxWriter(len(messages) + 2) // +2 keep alive messages
	serveRequest(ctx, writer, DefaultMessageToBytesConverter, stream, nil, 1*time.Millisecond, DefaultKeepAliveMessage)

	splitMinLen := len(messages) + 1 + 1 + 1 // initial keep alive message + one keep alive msg + one empty split entry at the end

	keepAliveMessageBytes := DefaultMessageToBytesConverter.Convert(DefaultKeepAliveMessage)

	keepAliveDetected := false
	split := bytes.SplitAfter(writer.Bytes(), []byte("\n\n"))
	assert.True(t, len(split) >= splitMinLen)
	assert.Equal(t, keepAliveMessageBytes, split[0])
	assert.Equal(t, DefaultMessageToBytesConverter.Convert(messages[0]), split[1])
	assert.Equal(t, DefaultMessageToBytesConverter.Convert(messages[1]), split[2])
	for i := len(messages) + 1; i < len(split)-1; i++ { // last split entry is always empty
		assert.Equal(t, keepAliveMessageBytes, split[i])
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

type maxWriter struct {
	i         int
	maxWrites int
	bytes.Buffer
}

func newMaxWriter(maxWrites int) *maxWriter {
	return &maxWriter{maxWrites: maxWrites}
}

func (m *maxWriter) Write(p []byte) (n int, err error) {
	if m.i >= m.maxWrites {
		return 0, fmt.Errorf("max writes reached")
	}

	m.i++
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
