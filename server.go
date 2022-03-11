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
		channelIDExtractor:             DefaultChannelIDExtractor,
		messageConverter:               DefaultMessageToBytesConverter,
		messageRepository:              nopMessageStore{},
		listenersKeepAliveInterval:     10 * time.Second,
		listenersKeepAliveMessageBytes: DefaultKeepAliveMessageBytes,
	}

	for _, setter := range optionSetters {
		setter(options)
	}

	return &Server{
		options:  options,
		channels: newChannels(options.messageConverter, options.messageRepository),
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
		s.options.listenersKeepAliveMessageBytes,
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
	stream := channel.stream()
	replayBytes := s.getReplayBytes(r, channelID)

	listenerConn.run(r.Context(), replayBytes, stream)
}

func (s *Server) getReplayBytes(r *http.Request, channelID string) [][]byte {
	lastID := r.Header.Get(headerLastEventID)
	if lastID == "" {
		return nil
	}

	var ret [][]byte
	replayMessages := s.options.messageRepository.GetReplay(channelID, lastID)
	for _, msg := range replayMessages {
		ret = append(ret, s.options.messageConverter(msg))
	}
	return ret
}

type ChannelIDExtractor func(*http.Request) (string, error)

var DefaultChannelIDExtractor ChannelIDExtractor = func(r *http.Request) (string, error) {
	return r.URL.Query().Get("channel"), nil
}

type ServerOptions struct {
	channelIDExtractor             ChannelIDExtractor
	messageConverter               MessageToBytesConverter
	messageRepository              MessageRepository
	listenersKeepAliveInterval     time.Duration
	listenersKeepAliveMessageBytes []byte
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

func WithMessageRepository(m MessageRepository) ServerOptionSetter {
	return func(options *ServerOptions) {
		if m != nil {
			options.messageRepository = m
		}
	}
}

func WithListenersKeepAliveInterval(d time.Duration) ServerOptionSetter {
	return func(options *ServerOptions) {
		options.listenersKeepAliveInterval = d
	}
}

func WithListenersKeepAliveMessageBytes(b []byte) ServerOptionSetter {
	return func(options *ServerOptions) {
		if len(b) > 0 {
			options.listenersKeepAliveMessageBytes = b
		}
	}
}
