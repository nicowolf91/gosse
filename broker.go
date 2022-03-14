package gosse

import (
	"github.com/imkira/go-observer"
	"sync"
)

type Broker interface {
	Publish(msg interface{})
	Subscribe() Stream
}

type ChannelBroker interface {
	Publish(channelID string, msg interface{})
	Broadcast(msg interface{})
	Subscribe(channelID string) Stream
}

type broker struct {
	p observer.Property
}

func NewBroker() Broker {
	return &broker{p: observer.NewProperty(nil)}
}

func (b *broker) Publish(msg interface{}) {
	b.p.Update(msg)
}

func (b *broker) Subscribe() Stream {
	return newStream(b.p.Observe())
}

type channelBroker struct {
	channelBrokers map[string]Broker
	m              sync.Mutex
}

func NewChannelBroker() ChannelBroker {
	return &channelBroker{
		channelBrokers: make(map[string]Broker),
	}
}

func (b *channelBroker) Publish(channelID string, msg interface{}) {
	b.getChannelBroker(channelID).Publish(msg)
}

func (b *channelBroker) Broadcast(msg interface{}) {
	for _, broker := range b.channelBrokers {
		broker.Publish(msg)
	}
}

func (b *channelBroker) Subscribe(channelID string) Stream {
	return b.getChannelBroker(channelID).Subscribe()
}

func (b *channelBroker) getChannelBroker(channelID string) Broker {
	b.m.Lock()
	defer b.m.Unlock()

	if b.channelBrokers[channelID] == nil {
		b.channelBrokers[channelID] = NewBroker()
	}
	return b.channelBrokers[channelID]
}
