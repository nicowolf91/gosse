package gosse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChannelBroker(t *testing.T) {
	messagesToSendOnChannel1 := []interface{}{
		"ch1",
		NewMessage().WithData([]byte("hello channel 1")),
	}
	messagesToSendOnChannel2 := []interface{}{
		NewMessage().WithData([]byte("hello channel 2")),
		NewMessage().WithData([]byte("hello channel 2")),
		"ch2",
	}
	messagesToBroadcast := []interface{}{
		NewMessage().WithEvent("e").WithData([]byte("data")),
		"broadcast",
	}

	expectedMessagesChannel1 := append(messagesToSendOnChannel1, messagesToBroadcast...)
	expectedMessagesChannel2 := append(messagesToSendOnChannel2, messagesToBroadcast...)

	broker := NewChannelBroker[string, any]()

	channel1Stream := broker.Subscribe("channel1")
	channel2Stream := broker.Subscribe("channel2")

	for _, msg := range messagesToSendOnChannel1 {
		broker.Publish("channel1", msg)
	}

	for _, msg := range messagesToSendOnChannel2 {
		broker.Publish("channel2", msg)
	}

	for _, msg := range messagesToBroadcast {
		broker.Broadcast(msg)
	}

	iCh1 := 0
	for channel1Stream.HasNext() {
		assert.Equal(t, expectedMessagesChannel1[iCh1], channel1Stream.Next())
		iCh1++
	}
	assert.Equal(t, len(expectedMessagesChannel1), iCh1)

	iCh2 := 0
	for channel2Stream.HasNext() {
		assert.Equal(t, expectedMessagesChannel2[iCh2], channel2Stream.Next())
		iCh2++
	}
	assert.Equal(t, len(expectedMessagesChannel2), iCh2)
}
