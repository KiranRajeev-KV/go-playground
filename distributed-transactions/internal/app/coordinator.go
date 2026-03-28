package app

import (
	"context"
	"fmt"
	"net"
	"time"

	"distributed-transactions/internal/coordinator"
	"distributed-transactions/internal/db"
	"distributed-transactions/pb"

	"google.golang.org/grpc"
)

func RunCoordinator(dbPath, inventoryAddr, paymentAddr string, port int, recover bool, commitTimeout time.Duration, protocol string) error {
	database, err := db.InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.InitSchema(ctx); err != nil {
		return fmt.Errorf("failed to init schema: %w", err)
	}

	clients, err := coordinator.CreateParticipantClients(inventoryAddr, paymentAddr)
	if err != nil {
		return fmt.Errorf("failed to create participant clients: %w", err)
	}

	service := coordinator.NewCoordinatorService(database, clients, recover, commitTimeout, protocol)
	server := coordinator.NewCoordinatorServer(service)

	if recover {
		fmt.Println("Running crash recovery...")
		if err := service.Recover(ctx); err != nil {
			fmt.Printf("Recovery error: %v\n", err)
		}
	}

	grpcServer := grpc.NewServer()
	pb.RegisterTransactionCoordinatorServer(grpcServer, server)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	fmt.Printf("Coordinator starting on port %d\n", port)
	return grpcServer.Serve(lis)
}
