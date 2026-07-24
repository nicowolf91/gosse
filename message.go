package gosse

import (
	"bytes"
	"strconv"
	"time"
)

type Messager interface {
	Event() string
	Data() []byte
	ID() string
	Retry() time.Duration
}

type MessageStorer interface {
	Store(channelID string, msg Messager)
	StoreBroadcast(msg Messager)
}

type MessageReplayer interface {
	GetReplay(channelID, lastSeenMessageID string) []Messager
}

type MessageToBytesConverter interface {
	Convert(Messager) []byte
}

var DefaultMessageToBytesConverter = &defaultMessageToBytesConverter{}

type defaultMessageToBytesConverter struct{}

func (d *defaultMessageToBytesConverter) Convert(msg Messager) []byte {
	if msg == nil {
		return nil
	}

	event := msg.Event()
	data := msg.Data()
	id := msg.ID()
	retry := msg.Retry()

	estimate := len(event) + len(data) + len(id) + 24
	var retryStr string
	if retry > 0 {
		retryStr = strconv.FormatInt(retry.Milliseconds(), 10)
		estimate += len(retryStr)
	}

	buf := bytes.NewBuffer(make([]byte, 0, estimate))

	if len(event) > 0 {
		buf.WriteString("event: ")
		buf.WriteString(event)
		buf.WriteByte('\n')
	}

	if len(data) > 0 {
		start := 0
		for i := 0; i <= len(data); i++ {
			if i == len(data) || data[i] == '\n' || data[i] == '\r' {
				if i > start {
					line := data[start:i]
					if line[0] == ':' {
						buf.Write(line)
					} else {
						buf.WriteString("data: ")
						buf.Write(line)
					}
					buf.WriteByte('\n')
				}
				if i < len(data) && data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
					i++
				}
				start = i + 1
			}
		}
	}

	if len(id) > 0 {
		buf.WriteString("id: ")
		buf.WriteString(id)
		buf.WriteByte('\n')
	}

	if retry > 0 {
		buf.WriteString("retry: ")
		buf.WriteString(retryStr)
		buf.WriteByte('\n')
	}

	buf.WriteByte('\n')
	return buf.Bytes()
}

type Message struct {
	event string
	data  []byte
	id    string
	retry time.Duration
}

func NewMessage(valueSetters ...MessageValueSetter) *Message {
	ret := &Message{}
	for _, setter := range valueSetters {
		setter(ret)
	}
	return ret
}

func (m *Message) WithEvent(e string) *Message {
	WithEvent(e)(m)
	return m
}

func (m *Message) WithData(d []byte) *Message {
	WithData(d)(m)
	return m
}

func (m *Message) WithID(id string) *Message {
	WithID(id)(m)
	return m
}

func (m *Message) WithRetry(d time.Duration) *Message {
	WithRetry(d)(m)
	return m
}

func (m *Message) Event() string {
	return m.event
}

func (m *Message) SetEvent(e string) {
	m.event = e
}

func (m *Message) Data() []byte {
	return m.data
}

func (m *Message) SetData(d []byte) {
	m.data = d
}

func (m *Message) ID() string {
	return m.id
}

func (m *Message) SetID(id string) {
	m.id = id
}

func (m *Message) Retry() time.Duration {
	return m.retry
}

func (m *Message) SetRetry(d time.Duration) {
	m.retry = d
}

type MessageValueSetter func(*Message)

func WithEvent(e string) MessageValueSetter {
	return func(msg *Message) {
		msg.SetEvent(e)
	}
}

func WithData(d []byte) MessageValueSetter {
	return func(msg *Message) {
		msg.SetData(d)
	}
}

func WithID(id string) MessageValueSetter {
	return func(msg *Message) {
		msg.SetID(id)
	}
}

func WithRetry(d time.Duration) MessageValueSetter {
	return func(msg *Message) {
		msg.SetRetry(d)
	}
}

var DefaultKeepAliveMessage = NewMessage().WithData([]byte(": "))

type nopMessageStorer struct{}

func (n nopMessageStorer) Store(channelID string, msg Messager) {}

func (n nopMessageStorer) StoreBroadcast(msg Messager) {}

type nopMessageReplayer struct{}

func (n nopMessageReplayer) GetReplay(channelID, lastSeenMessageID string) []Messager { return nil }
