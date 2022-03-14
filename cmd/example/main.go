package main

import (
	"fmt"
	"github.com/nicowolf91/gosse"
	"log"
	"net/http"
	"time"
)

func main() {
	s := gosse.NewServer()

	go func() {
		i := 1
		for range time.Tick(1 * time.Second) {
			if i%2 == 0 {
				msg := gosse.NewMessage(gosse.WithData([]byte(fmt.Sprintf("broadcast: bar %d", i))))
				s.Broadcast(msg)
			} else {
				msg := gosse.NewMessage(
					gosse.WithEvent("test"),
					gosse.WithData([]byte(fmt.Sprintf("bar %d", i))),
				)
				s.Publish("ch1", msg)
			}
			i++
		}
	}()

	if err := http.ListenAndServe(":13337", cors(s)); err != nil {
		log.Fatalln(err)
	}
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}
