package gosse

import (
	"net/http"
	"time"
)

type Server struct {
	channelIDExtractor         ChannelIDExtractor
	messageConverter           MessageToBytesConverter
	messageBroker              ChannelBroker[string, Messager]
	messageStorer              MessageStorer
	messageReplayer            MessageReplayer
	listenersAdditionalHeader  http.Header
	listenersKeepAliveInterval time.Duration
	listenersKeepAliveMessage  Messager
}

func NewServer(optionSetters ...ServerOptionSetter) *Server {
	ret := &Server{
		channelIDExtractor:         DefaultChannelIDExtractor,
		messageConverter:           DefaultMessageToBytesConverter,
		messageBroker:              NewChannelBroker[string, Messager](),
		messageStorer:              nopMessageStorer{},
		messageReplayer:            nopMessageReplayer{},
		listenersAdditionalHeader:  http.Header{},
		listenersKeepAliveInterval: 10 * time.Second,
		listenersKeepAliveMessage:  DefaultKeepAliveMessage,
	}

	for _, setter := range optionSetters {
		setter(ret)
	}

	return ret
}

func (s *Server) Publish(channelID string, msg Messager) {
	if msg == nil {
		return
	}
	s.messageBroker.Publish(channelID, msg)
	s.messageStorer.Store(channelID, msg)
}

func (s *Server) Broadcast(msg Messager) {
	if msg == nil {
		return
	}
	s.messageBroker.Broadcast(msg)
	s.messageStorer.StoreBroadcast(msg)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	channelID, err, httpErrorCode := s.channelIDExtractor.ExtractChannelID(r)
	if err != nil {
		http.Error(w, err.Error(), httpErrorCode)
		return
	}

	sseRequest, err := NewRequest(w, r, s.listenersAdditionalHeader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	replayMessages := s.messageReplayer.GetReplay(channelID, sseRequest.LastEventID)
	stream := s.messageBroker.Subscribe(channelID)

	serveRequest(
		r.Context(),
		sseRequest,
		s.messageConverter,
		stream,
		replayMessages,
		s.listenersKeepAliveInterval,
		s.listenersKeepAliveMessage,
	)
}

type ChannelIDExtractor interface {
	ExtractChannelID(r *http.Request) (string, error, int)
}

var DefaultChannelIDExtractor = defaultChannelIDExtractor{}

type defaultChannelIDExtractor struct{}

func (d defaultChannelIDExtractor) ExtractChannelID(r *http.Request) (string, error, int) {
	return r.URL.Query().Get("channel"), nil, http.StatusOK
}

type ServerOptionSetter func(*Server)

func WithChannelIDExtractor(g ChannelIDExtractor) ServerOptionSetter {
	return func(server *Server) {
		if g != nil {
			server.channelIDExtractor = g
		}
	}
}

func WithMessageToBytesConverter(m MessageToBytesConverter) ServerOptionSetter {
	return func(server *Server) {
		if m != nil {
			server.messageConverter = m
		}
	}
}

func WithChannelMessageBroker(b ChannelBroker[string, Messager]) ServerOptionSetter {
	return func(server *Server) {
		if b != nil {
			server.messageBroker = b
		}
	}
}

func WithMessageStorer(m MessageStorer) ServerOptionSetter {
	return func(server *Server) {
		if m != nil {
			server.messageStorer = m
		}
	}
}

func WithMessageReplayer(m MessageReplayer) ServerOptionSetter {
	return func(server *Server) {
		if m != nil {
			server.messageReplayer = m
		}
	}
}

func WithListenersAdditionalHeader(h http.Header) ServerOptionSetter {
	return func(server *Server) {
		server.listenersAdditionalHeader = h
	}
}

func WithListenersKeepAliveInterval(d time.Duration) ServerOptionSetter {
	return func(server *Server) {
		server.listenersKeepAliveInterval = d
	}
}

func WithListenersKeepAliveMessage(msg Messager) ServerOptionSetter {
	return func(server *Server) {
		if msg != nil {
			server.listenersKeepAliveMessage = msg
		}
	}
}
