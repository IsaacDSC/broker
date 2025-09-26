package main

import (
	"fmt"
	"time"

	"github.com/IsaacDSC/broker/broker"
	"github.com/IsaacDSC/broker/pub"
	sub "github.com/IsaacDSC/broker/sub"
	"github.com/google/uuid"
)

func main() {
	// client := broker.NewClient(broker.Config{
	// 	Addr:    "localhost:6379",
	// 	AppName: "app-name",
	// })
	client := broker.NewRdbClient(*broker.DefaultRedisConfig("app-name"))
	go SetupConsumer("🔵instance1", client)
	go SetupConsumer("🟢instance2", client)
	SetupProducer(client)
}

func SetupProducer(client *broker.Client) {
	broker := pub.NewBroker(client)
	for range 10 {
		func() {
			broker.Publish("event1", map[string]any{"id": uuid.New(), "user": "isaac"})
			// log.Println("Publisher event1")
		}()

		func() {
			broker.Publish("event2", map[string]any{"id": uuid.New(), "user": "raissa"})
			// log.Println("Publisher event2")
		}()

		func() {
			broker.Publish("event3", map[string]any{"id": uuid.New(), "user": "raquel"})
			// log.Println("Publisher event3")
		}()

		time.Sleep(time.Millisecond * 100)
	}
}

func SetupConsumer(instancePrefix string, client *broker.Client) {
	broker := sub.NewSubscribe(client).
		WithSubscriber(
			"event1", func(subctx sub.Ctx) error {
				fmt.Println(instancePrefix, "Handling event1", subctx.GetPayload())
				return nil
			},
		).
		WithSubscriber(
			"event2", func(subctx sub.Ctx) error {
				fmt.Println(instancePrefix, "Handling event2", subctx.GetPayload())
				return nil
			},
		)

	fmt.Println("[*] Starting listener...")
	if err := broker.Listener(); err != nil {
		fmt.Printf("Error starting listener: %v\n", err)
		return
	}
}
