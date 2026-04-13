package main

import (
	"fmt"
	"log"

	"github.com/thkx/agent/config"
	"github.com/thkx/agent/distributed"
	"github.com/thkx/agent/runtime"
)

func main() {
	fmt.Println("=== Example 09: Distributed Worker Architecture ===")
	fmt.Println("")

	// Create coordinator
	coordinator := distributed.NewCoordinator("coordinator-1")
	fmt.Println("✓ Created coordinator")

	// Create 2 worker nodes
	for i := 1; i <= 2; i++ {
		nodeID := fmt.Sprintf("worker-%d", i)
		transport := distributed.NewInMemoryTransport()

		cfg := config.Default()
		builder := config.NewBuilder(cfg)
		taskQ, resultQ, err := builder.BuildQueues()
		if err != nil {
			log.Fatal(err)
		}

		graphStore := runtime.NewMemoryGraphStore()
		_ = distributed.NewDistributedWorker(nodeID, taskQ, resultQ, graphStore, transport)

		if err := coordinator.RegisterNode(nodeID, transport); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("✓ Created worker: %s\n", nodeID)
	}

	fmt.Println("")
	fmt.Println("=== Distributed Architecture ===")
	fmt.Println("- Multiple worker nodes with transport layer")
	fmt.Println("- Coordinator-based task distribution")
	fmt.Println("- In-memory transport for local testing")
	fmt.Println("- Ready for gRPC/AMQP/NATS implementation")
}
