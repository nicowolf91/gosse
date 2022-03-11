package gosse

import (
	"net/http"
	"time"
)

type Server struct {
	options  *ServerOptions
	channels *channels
}

func NewServer(optionSetters ...ServerOptionSetter) *Server {
	options := &ServerOptions{
		channelIDExtractor:         DefaultChannelIDExtractor,
		messageConverter:           DefaultMessageToBytesConverter,
		messageStorer:              nopMessageStorer{},
		messageReplayer:            nopMessageReplayer{},
		listenersKeepAliveInterval: 10 * time.Second,
		listenersKeepAliveMessage:  DefaultKeepAliveMessage,
	}

	for _, setter := range optionSetters {
		setter(options)
	}

	return &Server{
		options:  options,
		channels: newChannels(options.messageConverter, options.messageStorer),
	}
}

func (s *Server) Broadcast(msg Messager) {
	s.channels.broadcast(msg)
}

func (s *Server) Send(channelID string, msg Messager) {
	s.channels.send(channelID, msg)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	listenerConn, err := newListenerConn(
		w,
		s.options.listenersKeepAliveInterval,
		s.options.messageConverter(s.options.listenersKeepAliveMessage),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	channelID, err := s.options.channelIDExtractor(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	channel := s.channels.get(channelID)

	replayMessages := s.getReplayMessages(r, channelID)
	stream := channel.stream()

	listenerConn.run(r.Context(), s.toReplayBytes(replayMessages), stream)
}

func (s *Server) getReplayMessages(r *http.Request, channelID string) []Messager {
	lastID := r.Header.Get(headerLastEventID)
	if lastID == "" {
		return nil
	}

	return s.options.messageReplayer.GetReplay(channelID, lastID)
}

func (s *Server) toReplayBytes(messages []Messager) [][]byte {
	var ret [][]byte
	for _, msg := range messages {
		ret = append(ret, s.options.messageConverter(msg))
	}

	return ret
}

type ChannelIDExtractor func(*http.Request) (string, error)

var DefaultChannelIDExtractor ChannelIDExtractor = func(r *http.Request) (string, error) {
	return r.URL.Query().Get("channel"), nil
}

type ServerOptions struct {
	channelIDExtractor         ChannelIDExtractor
	messageConverter           MessageToBytesConverter
	messageStorer              MessageStorer
	messageReplayer            MessageReplayer
	listenersKeepAliveInterval time.Duration
	listenersKeepAliveMessage  Messager
}

type ServerOptionSetter func(*ServerOptions)

func WithChannelIDExtractor(g ChannelIDExtractor) ServerOptionSetter {
	return func(options *ServerOptions) {
		if g != nil {
			options.channelIDExtractor = g
		}
	}
}

func WithMessageToBytesConverter(m MessageToBytesConverter) ServerOptionSetter {
	return func(options *ServerOptions) {
		if m != nil {
			options.messageConverter = m
		}
	}
}

func WithMessageStorer(m MessageStorer) ServerOptionSetter {
	return func(options *ServerOptions) {
		if m != nil {
			options.messageStorer = m
		}
	}
}

func WithMessageReplayer(m MessageReplayer) ServerOptionSetter {
	return func(options *ServerOptions) {
		if m != nil {
			options.messageReplayer = m
		}
	}
}

func WithListenersKeepAliveInterval(d time.Duration) ServerOptionSetter {
	return func(options *ServerOptions) {
		options.listenersKeepAliveInterval = d
	}
}

func WithListenersKeepAliveMessage(msg Messager) ServerOptionSetter {
	return func(options *ServerOptions) {
		if msg != nil {
			options.listenersKeepAliveMessage = msg
		}
	}
}
