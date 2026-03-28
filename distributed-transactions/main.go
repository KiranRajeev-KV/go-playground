package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"distributed-transactions/internal/app"
)

func main() {
	mode := flag.String("mode", "", "Service mode: coordinator, inventory, payment, or shipping")
	dbPath := flag.String("db", "data.db", "Path to SQLite database")
	port := flag.Int("port", 0, "Port to listen on (default depends on mode)")
	seed := flag.Bool("seed", false, "Seed database with sample data")
	recover := flag.Bool("recover", false, "Run crash recovery on startup")
	commitTimeout := flag.Duration("commit-timeout", 10*time.Second, "Timeout for commit operations (e.g., 10s, 30s)")

	inventoryAddr := flag.String("inventory-addr", "localhost:50052", "Inventory service address")
	paymentAddr := flag.String("payment-addr", "localhost:50053", "Payment service address")
	shippingAddr := flag.String("shipping-addr", "localhost:50054", "Shipping service address")

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
		if err := app.RunCoordinator(*dbPath, *inventoryAddr, *paymentAddr, *shippingAddr, p, *recover, *commitTimeout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "inventory":
		p := 50052
		if *port != 0 {
			p = *port
		}
		if err := app.RunParticipant(*dbPath, "inventory", p, *seed, *recover, *commitTimeout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "payment":
		p := 50053
		if *port != 0 {
			p = *port
		}
		if err := app.RunParticipant(*dbPath, "payment", p, *seed, *recover, *commitTimeout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "shipping":
		p := 50054
		if *port != 0 {
			p = *port
		}
		if err := app.RunParticipant(*dbPath, "shipping", p, *seed, *recover, *commitTimeout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("Error: invalid mode '%s'\n", *mode)
		flag.Usage()
		os.Exit(1)
	}
}
