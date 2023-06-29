![CI](https://github.com/nicowolf91/gosse/actions/workflows/go.yml/badge.svg)
[![codecov](https://codecov.io/gh/nicowolf91/gosse/branch/main/graph/badge.svg)](https://codecov.io/gh/nicowolf91/gosse)

# gosse - Go Server-Sent Events (SSE)
`gosse` is mainly intended as server-sided implementation of the SSE streaming concept.
It comes with some (convenience) features like:

* out-of-the-box use with reasonable defaults
* options pattern to easily customize the behavior of the server
* channel concept to send events/messages only to certain clients

## Example
The following code snippets show a fully working use case of `gosse`. Also take a look at the [example files](cmd/example_sse).

### Client
The [client](cmd/example_sse/test.html) (usually a web browser) connects to the backend server via the `EventSource` interface. It subscribes
to `channel_1` and prints the messages of type `message` (default type) and `custom_event` to the console log.

### Server
The [server implementation](cmd/example_sse/main.go) listens to and serves requests on port `13337` and allows CORS requests from any source.
It alternately sends messages/events to
* all subscribed clients via `Broadcast` with the default event type
* all clients subscribed to `channel_1` with the custom event type `custom_event`
* all clients subscribed to `channel_2` with the custom event type `custom_event`

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
The option `WithChannelIDExtractor` lets you set a custom implementation of the `ChannelIDExtractor` interface. 


Default: Implementation that extracts the query parameter `channel` from the request and returns a `nil` error and
`http.StatusOK` as http error code (only relevant when an error is returned).

### Message To Bytes Converter
The option `WithMessageToBytesConverter` allows you to define the way Messages are converted to the byte slice sent
to the client.

Default: [Implementation](message.go#L33)

### Channel Broker
The option `WithChannelMessageBroker` lets you use your custom implementation of the `ChannelBroker` interface with `ChannelIdType` as `string` and `MessageType` as `Messager`.

Default: [Implementation](channel_broker.go)

### Message Store
The `WithMessageStorer` option allows the usage of a custom implementation of the `MessageStorer` interface.

Default: Nop [implementation](message.go#L165) storing no messages for replay.

### Message Replayer
The option `WithMessageReplayer` lets you define which messages are to be replayed for a client sending its last event id by implementing the `MessageReplayer` interface.

Default: Nop [implementation](message.go#L171) returning no messages.

### Additional Headers
The `WithListenersAdditionalHeader` option lets you define http headers that are sent to the client additionally
to the default key `Content-Type` with value `text/event-stream` indicating SSE streaming.

Default: empty

### Listeners Keep Alive Interval
The `WithListenersKeepAliveInterval` option allows you to define the time interval that is used to send periodic
keep alive messages to a connected client. The keep alive interval only triggers when no other message is sent to
the client in the given time span.

Default: 10 seconds.

### Listeners Keep Alive Message
The option `WithListenersKeepAliveMessage` lets you define the keep alive message content.

Default: [Simple message](message.go#L163) containing an empty comment line.