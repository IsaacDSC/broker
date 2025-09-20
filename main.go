package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/IsaacDSC/broker/broker"
	"github.com/IsaacDSC/broker/config"
	"github.com/IsaacDSC/broker/load"
	"github.com/IsaacDSC/broker/pub"
	sub "github.com/IsaacDSC/broker/sub"
	"github.com/google/uuid"
)

func main() {
	// Permite escolher o tipo de implementação via variável de ambiente
	// Exemplo: BROKER_MODE=redis go run main.go
	// Exemplo: BROKER_MODE=memory go run main.go
	// Exemplo: BROKER_MODE=hybrid go run main.go
	mode := getEnvString("BROKER_MODE", "memory")
	redisAddr := getEnvString("REDIS_ADDR", "localhost:6379")

	var client *broker.Client
	var err error

	switch mode {
	case "redis":
		fmt.Println("🔄 Starting broker with Redis backend...")
		client, err = broker.NewRedisClient("app-name", redisAddr)
	case "hybrid":
		fmt.Println("🔄 Starting broker with Hybrid backend (Redis Queue + Memory Balancer)...")
		client, err = broker.NewHybridClient("app-name", redisAddr)
	case "memory":
		fallthrough
	default:
		fmt.Println("🔄 Starting broker with Memory backend...")
		client, err = broker.NewMemoryClient("app-name")
	}

	if err != nil {
		log.Fatalf("Failed to create broker client: %v", err)
	}

	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Error closing client: %v", err)
		}
	}()

	// Imprime configuração atual
	client.PrintConfig()

	// Verifica saúde dos componentes
	if err := client.Health(); err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	fmt.Println("✅ All components are healthy")

	// Inicia consumidores em goroutines separadas
	go SetupConsumer("instance1", client)
	go SetupConsumer("instance2", client)

	// Pequena pausa para os consumidores se registrarem
	time.Sleep(100 * time.Millisecond)

	// Mostra estatísticas iniciais
	showStats(client)

	// Inicia produção de mensagens
	SetupProducer(client)

	// Mostra estatísticas finais
	time.Sleep(500 * time.Millisecond)
	showStats(client)
}

func SetupProducer(client *broker.Client) {
	broker := pub.NewBroker(client)

	fmt.Println("\n📤 Starting message production...")

	for i := 0; i < 10; i++ {
		// Event 1
		err := broker.Publish("event1", map[string]any{
			"id":        uuid.New(),
			"user":      "isaac",
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

func SetupConsumer(instancePrefix string, client *broker.Client) {
	subscriber := sub.NewSubscribe(client).
		WithSubscriber(
			"event1", func(subctx sub.Ctx) error {
				payload := subctx.GetPayload()
				fmt.Printf("🟢 %s Handling event1: %v\n", instancePrefix, payload)

				// Simula processamento
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		).
		WithSubscriber(
			"event2", func(subctx sub.Ctx) error {
				payload := subctx.GetPayload()
				fmt.Printf("🔵 %s Handling event2: %v\n", instancePrefix, payload)

				// Simula processamento
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		)

	fmt.Printf("🎧 [%s] Starting listener (ID: %s)...\n", instancePrefix, subscriber.GetSubscriberID().String()[:8])

	// Configura peso do subscriber se necessário
	if instancePrefix == "instance1" {
		subscriber.SetWeight(2) // instance1 tem peso maior
	}

	if err := subscriber.Listener(); err != nil {
		fmt.Printf("❌ Error starting listener for %s: %v\n", instancePrefix, err)
		return
	}
}

func showStats(client *broker.Client) {
	stats, err := client.GetStats()
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
		return
	}

	fmt.Println("\n📊 === Broker Statistics ===")
	for key, value := range stats {
		fmt.Printf("📈 %s: %v\n", key, value)
	}
	fmt.Println("=========================\n")
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Exemplo de uso avançado com configuração customizada
func exampleCustomConfig() {
	// Configuração customizada
	cfg := &config.Config{
		AppName: "my-custom-app",
		Queue: &config.QueueConfig{
			Type:   "redis",
			Prefix: "custom_queue",
			Redis: &config.RedisConfig{
				Addr:         "localhost:6379",
				Password:     "",
				DB:           1,
				PoolSize:     20,
				MaxRetries:   5,
				MinIdleConns: 5,
			},
		},
		Balancer: &config.BalancerConfig{
			Type:          "redis",
			Strategy:      load.StrategyWeighted,
			HeartbeatTTL:  45 * time.Second,
			CleanupTicker: 120 * time.Second,
			Prefix:        "custom_balancer",
			Redis: &config.RedisConfig{
				Addr:         "localhost:6379",
				Password:     "",
				DB:           2,
				PoolSize:     15,
				MaxRetries:   3,
				MinIdleConns: 3,
			},
		},
	}

	client, err := broker.NewClientWithConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to create custom client: %v", err)
	}
	defer client.Close()

	fmt.Println("✅ Custom configuration client created successfully!")
	client.PrintConfig()
}
