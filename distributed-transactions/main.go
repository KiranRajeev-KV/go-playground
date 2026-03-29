package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"distributed-transactions/internal/app"
)

func main() {
	mode := flag.String("mode", "", "Service mode: coordinator, inventory, payment, or client")
	dbPath := flag.String("db", "data.db", "Path to SQLite database")
	port := flag.Int("port", 0, "Port to listen on (default depends on mode)")
	seed := flag.Bool("seed", false, "Seed database with sample data")
	recover := flag.Bool("recover", false, "Run crash recovery on startup")
	commitTimeout := flag.Duration("commit-timeout", 10*time.Second, "Timeout for commit operations (e.g., 10s, 30s)")
	idempotencyTTL := flag.Duration("idempotency-ttl", 60*time.Second, "TTL for idempotency cache (e.g., 60s)")
	protocol := flag.String("protocol", "2pc", "Protocol: 2pc or 3pc")
	chaos := flag.String("chaos", "none", "Chaos simulation (e.g., inventory-drop-prepare, payment-delay-commit)")
	chaosDelayDuration := flag.Duration("chaos-delay-duration", 15*time.Second, "Delay duration for chaos delay action")

	inventoryAddr := flag.String("inventory-addr", "localhost:50052", "Inventory service address")
	paymentAddr := flag.String("payment-addr", "localhost:50053", "Payment service address")

	tps := flag.Int("tps", 10, "Target transactions per second (for client mode)")
	duration := flag.Duration("duration", 30*time.Second, "Test duration (for client mode)")
	warmup := flag.Duration("warmup", 5*time.Second, "Warmup period (for client mode)")
	coordinatorAddr := flag.String("coordinator-addr", "localhost:50051", "Coordinator address (for client mode)")

	flag.Parse()

	if *mode == "" {
		fmt.Println("Error: -mode flag is required")
		flag.Usage()
		os.Exit(1)
	}

	switch *mode {
	case "coordinator":
		p := 50051
		if *port != 0 {
			p = *port
		}
		if err := app.RunCoordinator(*dbPath, *inventoryAddr, *paymentAddr, p, *recover, *commitTimeout, *protocol, *chaos); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "inventory":
		p := 50052
		if *port != 0 {
			p = *port
		}
		if err := app.RunParticipant(*dbPath, "inventory", p, *seed, *recover, *commitTimeout, *idempotencyTTL, *chaos, *chaosDelayDuration); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "payment":
		p := 50053
		if *port != 0 {
			p = *port
		}
		if err := app.RunParticipant(*dbPath, "payment", p, *seed, *recover, *commitTimeout, *idempotencyTTL, *chaos, *chaosDelayDuration); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "client":
		if err := app.RunClient(*coordinatorAddr, *tps, *duration, *warmup); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("Error: invalid mode '%s'\n", *mode)
		flag.Usage()
		os.Exit(1)
	}
}
