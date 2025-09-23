package main

import (
	"fmt"
	"os"
	"time"

	"github.com/IsaacDSC/broker/broker"
	"github.com/IsaacDSC/broker/sub"
)

func main() {

	args := os.Args
	fmt.Println(args[1])
	if len(args) < 2 {
		fmt.Println("Usage: consumer <1 or 2>")
		os.Exit(1)
	}

	client := broker.NewRdbClient(*broker.DefaultRedisConfig("app-name"))

	const instancePrefix = "instance-%s"
	mapper := map[string]string{"1": "🟢", "2": "🔵", "3": "🔴"}

	subscriber := sub.NewSubscribe(client).
		WithSubscriber(
			"event1", func(subctx sub.Ctx) error {
				payload := subctx.GetPayload()
				prefix := fmt.Sprintf(instancePrefix, args[1])
				fmt.Printf("%s %s Handling event1: %v\n", mapper[args[1]], prefix, payload)

				// Simula processamento
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		).
		WithSubscriber(
			"event2", func(subctx sub.Ctx) error {
				payload := subctx.GetPayload()
				prefix := fmt.Sprintf(instancePrefix, args[1])
				fmt.Printf("%s %s Handling event2: %v\n", mapper[args[1]], prefix, payload)

				// Simula processamento
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		)

	if err := subscriber.Listener(); err != nil {
		fmt.Printf("❌ Error starting listener for %s: %v\n", instancePrefix, err)
		return
	}
}
