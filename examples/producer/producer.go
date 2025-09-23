package main

import (
	"fmt"
	"log"
	"time"

	"github.com/IsaacDSC/broker/broker"
	"github.com/google/uuid"
)

func main() {
	broker := broker.NewRdbClient(*broker.DefaultRedisConfig("app-name"))
	// broker := pub.NewBroker(client)

	fmt.Println("\n📤 Starting message production...")

	for i := range 10 {
		// Event 1
		err := broker.Publish("event1", map[string]any{
			"id":        uuid.New(),
			"user":      "isaac",
			"iteration": i,
		})
		if err != nil {
			log.Printf("Error publishing event1: %v", err)
		}

		err = broker.Publish("event1", map[string]any{
			"id":        uuid.New(),
			"user":      "isaacdsc",
			"iteration": i,
		})
		if err != nil {
			log.Printf("Error publishing event1: %v", err)
		}

		// Event 2
		err = broker.Publish("event2", map[string]any{
			"id":        uuid.New(),
			"user":      "raissa",
			"iteration": i,
		})
		if err != nil {
			log.Printf("Error publishing event2: %v", err)
		}

		// Event 3 (não tem subscriber - deve mostrar log)
		err = broker.Publish("event3", map[string]any{
			"id":        uuid.New(),
			"user":      "raquel",
			"iteration": i,
		})
		if err != nil {
			log.Printf("Error publishing event3: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("✅ Message production completed")
}
