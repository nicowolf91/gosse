package gosse

import (
	"github.com/imkira/go-observer"
	"sync"
)

type channels struct {
	messageConverter MessageToBytesConverter
	messageStore     MessageStorer
	list             map[string]*channel
	m                sync.RWMutex
}

func newChannels(
	messageConverter MessageToBytesConverter,
	messageStore MessageStorer,
) *channels {
	return &channels{
		messageConverter: messageConverter,
		messageStore:     messageStore,
		list:             make(map[string]*channel),
	}
}

func (g *channels) broadcast(msg Messager) {
	for channelID := range g.list {
		g.send(channelID, msg)
	}
}

func (g *channels) send(channelID string, msg Messager) {
	if msg == nil {
		return
	}

	g.messageStore.Store(channelID, msg)
	g.sendBytes(channelID, g.messageConverter(msg))
}

func (g *channels) sendBytes(channelID string, msgBytes []byte) {
	if g.list[channelID] == nil {
		return
	}
	g.list[channelID].send(msgBytes)
}

func (g *channels) get(channelID string) *channel {
	g.m.Lock()
	defer g.m.Unlock()

	if g.list[channelID] == nil {
		g.list[channelID] = newChannel()
	}
	return g.list[channelID]
}

type channel struct {
	p observer.Property
}

func newChannel() *channel {
	return &channel{
		p: observer.NewProperty(nil),
	}
}

func (g *channel) send(msgBytes []byte) {
	if len(msgBytes) == 0 {
		return
	}
	g.p.Update(msgBytes)
}

func (g *channel) stream() observer.Stream {
	return g.p.Observe()
}
