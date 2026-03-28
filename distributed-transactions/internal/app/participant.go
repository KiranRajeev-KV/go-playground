package app

import (
	"context"
	"fmt"
	"net"
	"time"

	"distributed-transactions/internal/db"
	"distributed-transactions/internal/middleware"
	"distributed-transactions/internal/participant"
	"distributed-transactions/pb"

	"google.golang.org/grpc"
)

func RunParticipant(dbPath, participantType string, port int, seed bool, recover bool, commitTimeout time.Duration, idempotencyTTL time.Duration) error {
	database, err := db.InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.InitSchema(ctx); err != nil {
		return fmt.Errorf("failed to init schema: %w", err)
	}

	if seed {
		if err := seedParticipant(ctx, database, participantType); err != nil {
			return fmt.Errorf("failed to seed: %w", err)
		}
		fmt.Printf("Seeded %s data\n", participantType)
	}

	var pType participant.ParticipantType
	switch participantType {
	case "inventory":
		pType = participant.ParticipantInventory
	case "payment":
		pType = participant.ParticipantPayment
	case "shipping":
		pType = participant.ParticipantShipping
	default:
		return fmt.Errorf("invalid participant type: %s", participantType)
	}

	server := participant.NewParticipantServer(database, pType, recover, commitTimeout)

	if recover {
		fmt.Println("Running crash recovery...")
		if err := server.Recover(ctx); err != nil {
			fmt.Printf("Recovery error: %v\n", err)
		}
	}

	idempotencyStore := middleware.NewIdempotencyStore(database, idempotencyTTL)
	idempotencyInterceptor := middleware.NewIdempotencyInterceptor(idempotencyStore, idempotencyTTL)

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(idempotencyInterceptor.Unary()),
	)
	pb.RegisterTransactionParticipantServer(grpcServer, server)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	fmt.Printf("%s participant starting on port %d\n", participantType, port)
	return grpcServer.Serve(lis)
}

func seedParticipant(ctx context.Context, database *db.DB, participantType string) error {
	seedData := db.DefaultSeedData()

	switch participantType {
	case "inventory":
		return database.SeedInventory(ctx, seedData.Inventory)
	case "payment":
		return database.SeedPaymentAccounts(ctx, seedData.PaymentAccounts)
	case "shipping":
		return database.SeedShippingAddresses(ctx, seedData.ShippingAddresses)
	default:
		return fmt.Errorf("unknown participant type: %s", participantType)
	}
}
