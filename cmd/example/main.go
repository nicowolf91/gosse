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
				msg := gosse.NewMessage().WithData([]byte(fmt.Sprintf("broadcast: %d", i)))
				s.Broadcast(msg)
			} else {
				msg := gosse.NewMessage().
					WithEvent("test").
					WithData([]byte(fmt.Sprintf("ch1: %d", i)))
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
