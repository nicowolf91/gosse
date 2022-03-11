package gosse

import (
	"bytes"
	"fmt"
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
}

type MessageReplayer interface {
	GetReplay(channelID, lastID string) []Messager
}

type MessageToBytesConverter func(Messager) []byte

var DefaultMessageToBytesConverter MessageToBytesConverter = func(msg Messager) []byte {
	if msg == nil {
		return nil
	}

	var ret []byte

	if len(msg.Event()) > 0 {
		ret = append(ret, []byte(fmt.Sprintf("event: %s\n", msg.Event()))...)
	}

	if len(msg.Data()) > 0 {
		dataCopy := make([]byte, len(msg.Data()))
		copy(dataCopy, msg.Data())

		bytes.ReplaceAll(dataCopy, []byte("\r\n"), []byte("\n"))

		for _, dataBytes := range bytes.FieldsFunc(dataCopy, func(r rune) bool { return r == '\n' || r == '\r' }) {
			if bytes.HasPrefix(dataBytes, []byte(":")) {
				ret = append(ret, []byte(fmt.Sprintf("%s\n", dataBytes))...)
			} else {
				ret = append(ret, []byte(fmt.Sprintf("data: %s\n", dataBytes))...)
			}
		}
	}

	if len(msg.ID()) > 0 {
		ret = append(ret, []byte(fmt.Sprintf("id: %s\n", msg.ID()))...)
	}

	if msg.Retry() > 0 {
		ret = append(ret, []byte(fmt.Sprintf("retry: %d\n", msg.Retry().Milliseconds()))...)
	}

	return append(ret, '\n')
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

func (m Message) Event() string {
	return m.event
}

func (m *Message) SetEvent(e string) {
	m.event = e
}

func (m Message) Data() []byte {
	return m.data
}

func (m *Message) SetData(d []byte) {
	m.data = d
}

func (m Message) ID() string {
	return m.id
}

func (m *Message) SetID(id string) {
	m.id = id
}

func (m Message) Retry() time.Duration {
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

type nopMessageStorer struct{}

func (n nopMessageStorer) Store(channelID string, msg Messager) {}

type nopMessageReplayer struct{}

func (n nopMessageReplayer) GetReplay(channelID, lastID string) []Messager { return nil }
