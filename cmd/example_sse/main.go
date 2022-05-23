package main

import (
	"fmt"
	"github.com/nicowolf91/gosse"
	"log"
	"net/http"
	"time"
)

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
