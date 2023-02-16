![CI](https://github.com/nicowolf91/gosse/actions/workflows/go.yml/badge.svg)
[![codecov](https://codecov.io/gh/nicowolf91/gosse/branch/main/graph/badge.svg)](https://codecov.io/gh/nicowolf91/gosse)

# gosse - Go Server-Sent Events (SSE)
`gosse` is mainly intended as server-sided implementation of the SSE streaming concept.
It comes with some (convenience) features like:

* out-of-the-box use with reasonable defaults
* options pattern to easily customize the behavior of the server
* channel concept to send events/messages only to certain clients
* brokers and streams implementation for simple Pub/Sub

## Example
The following code snippets show a fully working use case of `gosse`.

### Client
The client (usually a web browser) connects to the backend server via the `EventSource` interface. It subscribes
to `channel_1` and prints the messages of type `message` (default type) and `custom_event` to the console log.

```html
<script>
    // connect to the SSE backend on channel "channel_1" (set via query parameter "channel")
    const source = new EventSource('http://localhost:13337?channel=channel_1');

    // subscribe to data-only messages (events without specific type)
    source.addEventListener('message', function(e) {
        console.log('event type ->', e.type, '| data ->', e.data);
    }, false);

    // subscribe to the messages of the specific type "custom_event"
    source.addEventListener('custom_event', function(e) {
        console.log('event type ->', e.type, '| data ->', e.data);
    }, false);
</script>
```

### Server
The server implementation listens and server requests on port `13337` and allows CORS requests from any source.
It alternately sends messages/events to
* all subscribed clients via `Broadcast` with the default event type
* all clients subscribed to `channel_1` with the custom event type `custom_event`
* all clients subscribed to `channel_2` with the custom event type `custom_event`

```go
func main() {
	server := gosse.NewServer()

	go sendMessages(server)

	if err := http.ListenAndServe(":13337", cors(server)); err != nil {
		log.Fatalln(err)
	}
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}

func sendMessages(server *gosse.Server) {
	i := 1
	for range time.Tick(1 * time.Second) {
		var sender func(msg gosse.Messager)
		var message gosse.Messager

		switch remainder := i % 3; remainder {
		case 0:
			// message with default event type (data-only message)
			message = gosse.NewMessage().WithData([]byte(fmt.Sprintf("broadcast: %d", i)))
			// broadcast to all channels
			sender = server.Broadcast
		default: // remainder in (1, 2)
			channel := fmt.Sprintf("channel_%d", remainder)
			message = gosse.NewMessage().
				// message with specific event type "custom_event"
				WithEvent("custom_event").
				WithData([]byte(fmt.Sprintf("%s: %d", channel, i)))
			// send to "channel_x" (where x is the remainder)
			sender = func(msg gosse.Messager) {
				server.Publish(channel, msg)
			}
		}

		sender(message)
		i++
	}
}

```

### Output
When the server is running and the client connects successfully the developer console of your browser
should show an output like that (the client will only receive broadcast messages and messages for `channel_1`):

```
event type -> custom_event | data -> channel_1: 1
event type -> message | data -> broadcast: 3
event type -> custom_event | data -> channel_1: 4
event type -> message | data -> broadcast: 6
event type -> custom_event | data -> channel_1: 7
event type -> message | data -> broadcast: 9
event type -> custom_event | data -> channel_1: 10
event type -> message | data -> broadcast: 12
event type -> custom_event | data -> channel_1: 13
[...]
```

## Server Options
The server can be configured via options passed to the constructor.

### Channel ID Extractor
The option `WithChannelIDExtractor` expects an implementation of the following interface: 

```go
type ChannelIDExtractor interface {
	ExtractChannelID(r *http.Request) (string, error, int)
}
```

Default: Implementation that extracts the query parameter `channel` from the request and returns a `nil` error and
`http.StatusOK` as http error code (only relevant when an error is returned). If you want to introduce your own channel
extraction concept (e.g. take value from the request context) you can simply override the default implementation.

### Message to bytes converter
The option `WithMessageToBytesConverter` allows you to define the way Messages are converted to the byte slice sent
to the client.

### Channel broker
The option `WithChannelMessageBroker` lets you use your custom implementation of the channel broker interface:

```go
type ChannelBroker interface {
	Publish(channelID string, msg interface{})
	Broadcast(msg interface{})
	Subscribe(channelID string) Stream
}
```

### Message store
The `WithMessageStorer` option allows the usage of a custom implementation of the interface:

```go
type MessageStorer interface {
	Store(channelID string, msg Messager)
	StoreBroadcast(msg Messager)
}
```

Default: Nop implementation storing no messages for replay.

### Message replayer
The option `WithMessageReplayer` lets you define which messages are to be replayed for a client
sending its last event id by implementing the interface:

```go
type MessageReplayer interface {
	GetReplay(channelID, lastSeenMessageID string) []Messager
}
```

Default: Nop implementation returning no messages.

### Additional Headers
The `WithListenersAdditionalHeader` option lets you define http headers that are sent to the client additionally
to the default key `Content-Type` with value `text/event-stream` indicating SSE streaming.\
Default: empty

### Listeners Keep Alive Interval
The `WithListenersKeepAliveInterval` option allows you to define the time interval that is used to send periodic
keep alive messages to a connected client. The keep alive interval only triggers when no other message is sent to
the client in the given time span.\
Default: 10 seconds.

### Listeners Keep Alive Message
The option `WithListenersKeepAliveMessage` lets you define the keep alive message content.\
Default: Simple message containing an empty comment line.

## Brokers and Streams
`gosse` also provides a versatile Pub/Sub implementation (e.g. for communication between goroutines)

### Broker
The `Broker` interface defines the following methods:

```go
type Broker interface {
	Publish(msg interface{})
	Subscribe() Stream
}
```

One can simply `Publish` messages and `Subscribe` to the broker receiving a `Stream` instance.

### Channel Broker
The `ChannelBroker` interface defines the following methods:

```go
type ChannelBroker interface {
	Publish(channelID string, msg interface{})
	Broadcast(msg interface{})
	Subscribe(channelID string) Stream
}
```

One can either `Publish` messages for a certain channel or `Broadcast` messages to all subscribers irrespective of
the channels they subscribed to. Also, a client can `Subscribe` to a certain channel by using the `Stream` instance.

### Stream
The `Stream` interface is based on one of my favorite Go libraries
<a href="https://github.com/imkira/go-observer" target="_blank">go-observer</a>
and extends it to work with `context.Context`:

```go
type Stream interface {
	observer.Stream
	WaitNextCtx(ctx context.Context) (interface{}, error)
}

type observer.Stream interface {
	Value() interface{}
	Changes() chan struct{}
	Next() interface{}
	HasNext() bool
	WaitNext() interface{}
	Clone() Stream
}
```

`WaitNextCtx` accepts a context and waits for either the next value for the subscriptions or until the context
is canceled. If the context was canceled the stream returns the underlying `error`.

### Example
The following code snippet shows the usage of the `gosse` `Broker` and `Stream` interfaces:

```go
func main() {
	ctx, cancel := context.WithCancel(context.Background())

	broker := gosse.NewBroker()
	stream := broker.Subscribe()

	for i := 1; i <= 100; i++ {
		broker.Publish(i)
	}

	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	for {
		next, err := stream.WaitNextCtx(ctx)
		if err != nil {
			return
		}
		fmt.Println(next)
	}
}
```

The output shows the numbers from `1` to `100`. A subscriber receives all the values sent
via the `Broker` from the point `Subscribe` was called (i.e. as soon as the `Stream` instance is returned).
