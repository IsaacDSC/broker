package main

import (
	"fmt"
	"os"
	"time"

	"github.com/IsaacDSC/broker/broker"
)

func main() {

	args := os.Args
	fmt.Println(args[1])
	if len(args) < 2 {
		fmt.Println("Usage: consumer <1 or 2>")
		os.Exit(1)
	}

	client := broker.NewClient(*broker.DefaultRedisConfig("app-name"))

	defer client.Close()

	const instancePrefix = "instance-%s"
	mapper := map[string]string{"1": "🟢", "2": "🔵", "3": "🔴"}

	subscriber := client.
		Subscribe(
			"event1", func(subctx broker.Ctx) error {
				payload := subctx.GetPayload()
				prefix := fmt.Sprintf(instancePrefix, args[1])
				fmt.Printf("%s %s Handling event1: %v\n", mapper[args[1]], prefix, payload)

				// Simula processamento
				time.Sleep(1000 * time.Millisecond)
				return nil
			},
		).
		Subscribe(
			"event2", func(subctx broker.Ctx) error {
				payload := subctx.GetPayload()
				prefix := fmt.Sprintf(instancePrefix, args[1])
				fmt.Printf("%s %s Handling event2: %v\n", mapper[args[1]], prefix, payload)

				// Simula processamento
				time.Sleep(1000 * time.Millisecond)
				return nil
			},
		)

	if err := subscriber.Listener(); err != nil {
		fmt.Printf("❌ Error starting listener for %s: %v\n", instancePrefix, err)
		return
	}
}
