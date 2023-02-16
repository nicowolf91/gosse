package main

import (
	"context"
	"fmt"
	"github.com/nicowolf91/gosse"
	"time"
)

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
