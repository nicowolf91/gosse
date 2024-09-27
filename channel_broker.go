package gosse

import (
	"sync"

	"github.com/imkira/go-observer/v2"
)

type ChannelBroker[ChannelIdType comparable, MessageType any] interface {
	Publish(channelID ChannelIdType, msg MessageType)
	Broadcast(msg MessageType)
	Subscribe(channelID ChannelIdType) observer.Stream[MessageType]
}

type channelBroker[ChannelIdType comparable, MessageType any] struct {
	props map[ChannelIdType]observer.Property[MessageType]
	m     sync.RWMutex
}

func NewChannelBroker[ChannelIdType comparable, MessageType any]() ChannelBroker[ChannelIdType, MessageType] {
	return &channelBroker[ChannelIdType, MessageType]{
		props: make(map[ChannelIdType]observer.Property[MessageType]),
	}
}

func (b *channelBroker[ChannelIdType, MessageType]) Publish(channelID ChannelIdType, msg MessageType) {
	b.getChannelProp(channelID).Update(msg)
}

func (b *channelBroker[ChannelIdType, MessageType]) Broadcast(msg MessageType) {
	b.m.RLock()
	defer b.m.RUnlock()

	for _, prop := range b.props {
		prop.Update(msg)
	}
}

func (b *channelBroker[ChannelIdType, MessageType]) Subscribe(channelID ChannelIdType) observer.Stream[MessageType] {
	return b.getChannelProp(channelID).Observe()
}

func (b *channelBroker[ChannelIdType, MessageType]) getChannelProp(channelID ChannelIdType) observer.Property[MessageType] {
	b.m.Lock()
	defer b.m.Unlock()

	if b.props[channelID] == nil {
		var zeroVal MessageType
		b.props[channelID] = observer.NewProperty(zeroVal)
	}
	return b.props[channelID]
}
