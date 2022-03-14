package gosse

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestMessageConstructorInit(t *testing.T) {
	msg := NewMessage()

	assert.Equal(t, "", msg.Event())
	assert.Nil(t, msg.Data())
	assert.Equal(t, "", msg.ID())
	assert.Equal(t, time.Duration(0), msg.Retry())
}

func TestMessageConstructor(t *testing.T) {
	event := "test"
	data := []byte("foobar")
	id := "id1"
	retry := 10 * time.Millisecond

	msg := NewMessage(
		WithEvent(event),
		WithData(data),
		WithID(id),
		WithRetry(retry),
	)

	assert.Equal(t, event, msg.Event())
	assert.Equal(t, data, msg.Data())
	assert.Equal(t, id, msg.ID())
	assert.Equal(t, retry, msg.Retry())
}

func TestMessageSetters(t *testing.T) {
	event := "hello"
	data := []byte("world")
	id := "id2"
	retry := 1 * time.Hour

	msg := NewMessage()
	msg.SetEvent(event)
	msg.SetData(data)
	msg.SetID(id)
	msg.SetRetry(retry)

	assert.Equal(t, event, msg.Event())
	assert.Equal(t, data, msg.Data())
	assert.Equal(t, id, msg.ID())
	assert.Equal(t, retry, msg.Retry())
}

func TestMessageSetterChain(t *testing.T) {
	event := "baz"
	data := []byte("test")
	id := "id3"
	retry := 3 * time.Second

	msg := NewMessage().WithEvent(event).WithData(data).WithID(id).WithRetry(retry)

	assert.Equal(t, event, msg.Event())
	assert.Equal(t, data, msg.Data())
	assert.Equal(t, id, msg.ID())
	assert.Equal(t, retry, msg.Retry())
}

func TestDefaultMessageToBytesConverter(t *testing.T) {
	var testCases = map[string]struct {
		msg      Messager
		expected []byte
	}{
		"nil message": {
			msg:      nil,
			expected: nil,
		},
		"empty message": {
			msg:      NewMessage(),
			expected: []byte("\n"),
		},
		"event": {
			msg:      NewMessage().WithEvent("event1"),
			expected: []byte("event: event1\n\n"),
		},
		"data": {
			msg:      NewMessage().WithData([]byte("test 123")),
			expected: []byte("data: test 123\n\n"),
		},
		"comment data": {
			msg:      NewMessage().WithData([]byte(": comment")),
			expected: []byte(": comment\n\n"),
		},
		"id": {
			msg:      NewMessage().WithID("id321"),
			expected: []byte("id: id321\n\n"),
		},
		"retry": {
			msg:      NewMessage().WithRetry(100 * time.Millisecond),
			expected: []byte("retry: 100\n\n"),
		},
		"data with \\n": {
			msg:      NewMessage().WithData([]byte("hello\nworld")),
			expected: []byte("data: hello\ndata: world\n\n"),
		},
		"data with \\r": {
			msg:      NewMessage().WithData([]byte("hello\rworld")),
			expected: []byte("data: hello\ndata: world\n\n"),
		},
		"data with \\r\\n": {
			msg:      NewMessage().WithData([]byte("hello\r\nworld")),
			expected: []byte("data: hello\ndata: world\n\n"),
		},
		"event and data": {
			msg:      NewMessage().WithEvent("foo").WithData([]byte("bar\nbaz")),
			expected: []byte("event: foo\ndata: bar\ndata: baz\n\n"),
		},
		"event and id": {
			msg:      NewMessage().WithEvent("world").WithID("hello"),
			expected: []byte("event: world\nid: hello\n\n"),
		},
		"event, data and id": {
			msg:      NewMessage().WithEvent("add").WithData([]byte("1")).WithID("456"),
			expected: []byte("event: add\ndata: 1\nid: 456\n\n"),
		},
		"event, data, id an retry": {
			msg: NewMessage().
				WithEvent("e").
				WithData([]byte("data")).
				WithID("7").
				WithRetry(1 * time.Millisecond),
			expected: []byte("event: e\ndata: data\nid: 7\nretry: 1\n\n"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, DefaultMessageToBytesConverter.Convert(tc.msg))
		})
	}
}
