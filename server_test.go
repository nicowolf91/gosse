package gosse

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestServerConstructorDefaults(t *testing.T) {
	serverWithDefaultValues := NewServer(
		WithChannelIDExtractor(DefaultChannelIDExtractor),
		WithMessageToBytesConverter(DefaultMessageToBytesConverter),
		WithChannelMessageBroker(NewChannelBroker()),
		WithMessageStorer(nopMessageStorer{}),
		WithMessageReplayer(nopMessageReplayer{}),
		WithListenersAdditionalHeader(http.Header{}),
		WithListenersKeepAliveInterval(10*time.Second),
		WithListenersKeepAliveMessage(DefaultKeepAliveMessage),
	)

	server := NewServer()

	assert.Equal(t, server, serverWithDefaultValues)
	assert.Equal(t, DefaultChannelIDExtractor, serverWithDefaultValues.channelIDExtractor)
	assert.Equal(t, DefaultMessageToBytesConverter, serverWithDefaultValues.messageConverter)
	assert.Equal(t, NewChannelBroker(), serverWithDefaultValues.messageBroker)
	assert.Equal(t, nopMessageStorer{}, serverWithDefaultValues.messageStorer)
	assert.Equal(t, nopMessageReplayer{}, serverWithDefaultValues.messageReplayer)
	assert.Equal(t, http.Header{}, serverWithDefaultValues.listenersAdditionalHeader)
	assert.Equal(t, 10*time.Second, serverWithDefaultValues.listenersKeepAliveInterval)
	assert.Equal(t, DefaultKeepAliveMessage, serverWithDefaultValues.listenersKeepAliveMessage)
}

func TestServerConstructor(t *testing.T) {
	extractor := channelExtractorMock{}
	converter := messageConverterMock{}
	broker := newChannelBrokerMock()
	storer := newMessageStorerMock()
	replayer := &messageReplayerMock{}
	additionalHeader := http.Header{}
	additionalHeader.Set("test", "123")
	keepAliveInterval := 1337 * time.Millisecond
	keepAliveMessage := NewMessage().WithData([]byte(": stay awake!"))

	server := NewServer(
		WithChannelIDExtractor(extractor),
		WithMessageToBytesConverter(converter),
		WithChannelMessageBroker(broker),
		WithMessageStorer(storer),
		WithMessageReplayer(replayer),
		WithListenersAdditionalHeader(additionalHeader),
		WithListenersKeepAliveInterval(keepAliveInterval),
		WithListenersKeepAliveMessage(keepAliveMessage),
	)

	assert.Equal(t, extractor, server.channelIDExtractor)
	assert.Equal(t, converter, server.messageConverter)
	assert.Equal(t, broker, server.messageBroker)
	assert.Equal(t, storer, server.messageStorer)
	assert.Equal(t, replayer, server.messageReplayer)
	assert.Equal(t, additionalHeader, server.listenersAdditionalHeader)
	assert.Equal(t, keepAliveInterval, server.listenersKeepAliveInterval)
	assert.Equal(t, keepAliveMessage, server.listenersKeepAliveMessage)
}

func TestServer_PublishBroadcast(t *testing.T) {
	broker := newChannelBrokerMock()
	storer := newMessageStorerMock()
	server := NewServer(
		WithChannelMessageBroker(broker),
		WithMessageStorer(storer),
	)

	channelID := "chan1"
	published := NewMessage().WithData([]byte("pub"))
	broadcast := NewMessage().WithData([]byte("broad"))

	server.Publish(channelID, published)
	server.Broadcast(broadcast)

	assert.Len(t, broker.list[channelID], 1)
	assert.Len(t, broker.list, 1)
	assert.Len(t, broker.broadcasts, 1)
	assert.Equal(t, DefaultMessageToBytesConverter.Convert(published), broker.list[channelID][0])
	assert.Equal(t, DefaultMessageToBytesConverter.Convert(broadcast), broker.broadcasts[0])

	assert.Len(t, storer.list[channelID], 1)
	assert.Len(t, storer.list, 1)
	assert.Len(t, storer.broadcasts, 1)
	assert.Equal(t, published, storer.list[channelID][0])
	assert.Equal(t, broadcast, storer.broadcasts[0])
}

func TestServer_ServeHTTPNoFlusher(t *testing.T) {
	server := NewServer()
	request := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	responseWriter := newNoFlushMock()
	server.ServeHTTP(responseWriter, request.WithContext(ctx))
	assert.Equal(t, http.StatusInternalServerError, responseWriter.statusCode)
}

func TestServer_ServeHTTPChannelExtractorFail(t *testing.T) {
	server := NewServer(
		WithChannelIDExtractor(channelExtractorMock{
			ret:  "",
			err:  fmt.Errorf("fail"),
			code: http.StatusTeapot,
		}),
	)
	request := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request.WithContext(ctx))
	assert.Equal(t, http.StatusTeapot, recorder.Code)
}

func TestServer_ServeHTTP(t *testing.T) {
	requestUrl, err := url.Parse("http://localhost?channel=c1")
	assert.NoError(t, err)

	request := httptest.NewRequest("GET", requestUrl.String(), nil)
	request.Header.Set("Last-Event-ID", "1337")
	ctx, cancel := context.WithCancel(request.Context())
	cancel()

	replayMessages := []Messager{
		NewMessage().WithData([]byte("replay 1")),
		NewMessage().WithData([]byte("replay 2")),
	}
	replayer := &messageReplayerMock{ret: replayMessages}
	server := NewServer(WithMessageReplayer(replayer))

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request.WithContext(ctx))

	var expectedBytes []byte
	for _, msg := range replayMessages {
		expectedBytes = append(expectedBytes, DefaultMessageToBytesConverter.Convert(msg)...)
	}

	assert.Equal(t, expectedBytes, recorder.Body.Bytes())
	assert.Equal(t, "c1", replayer.channelID)
	assert.Equal(t, "1337", replayer.lastSeenMessageID)
}

type channelExtractorMock struct {
	ret  string
	err  error
	code int
}

func (c channelExtractorMock) ExtractChannelID(r *http.Request) (string, error, int) {
	return c.ret, c.err, c.code
}

type messageConverterMock struct{}

func (m messageConverterMock) Convert(msg Messager) []byte { return nil }

type channelBrokerMock struct {
	list       map[string][]interface{}
	broadcasts []interface{}
	m          sync.Mutex
}

func newChannelBrokerMock() *channelBrokerMock {
	return &channelBrokerMock{
		list: make(map[string][]interface{}),
	}
}

func (c *channelBrokerMock) Publish(channelID string, msg interface{}) {
	c.m.Lock()
	defer c.m.Unlock()
	c.list[channelID] = append(c.list[channelID], msg)
}

func (c *channelBrokerMock) Broadcast(msg interface{}) {
	c.m.Lock()
	defer c.m.Unlock()
	c.broadcasts = append(c.broadcasts, msg)
}

func (c *channelBrokerMock) Subscribe(channelID string) Stream { return nopStreamMock{} }

type messageStorerMock struct {
	list       map[string][]interface{}
	broadcasts []interface{}
	m          sync.Mutex
}

func newMessageStorerMock() *messageStorerMock {
	return &messageStorerMock{
		list: make(map[string][]interface{}),
	}
}

func (m *messageStorerMock) Store(channelID string, msg Messager) {
	m.m.Lock()
	defer m.m.Unlock()
	m.list[channelID] = append(m.list[channelID], msg)
}

func (m *messageStorerMock) StoreBroadcast(msg Messager) {
	m.m.Lock()
	defer m.m.Unlock()
	m.broadcasts = append(m.broadcasts, msg)
}

type messageReplayerMock struct {
	ret                          []Messager
	channelID, lastSeenMessageID string
}

func (m *messageReplayerMock) GetReplay(channelID, lastSeenMessageID string) []Messager {
	m.channelID = channelID
	m.lastSeenMessageID = lastSeenMessageID
	return m.ret
}
