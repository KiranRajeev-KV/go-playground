package app

import (
	"context"
	"fmt"
	"net"

	"distributed-transactions/internal/db"
	"distributed-transactions/internal/participant"
	"distributed-transactions/pb"

	"google.golang.org/grpc"
)

func RunParticipant(dbPath, participantType string, port int) error {
	database, err := db.InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.InitSchema(ctx); err != nil {
		return fmt.Errorf("failed to init schema: %w", err)
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

	server := participant.NewParticipantServer(database, pType)

	grpcServer := grpc.NewServer()
	pb.RegisterTransactionParticipantServer(grpcServer, server)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	fmt.Printf("%s participant starting on port %d\n", participantType, port)
	return grpcServer.Serve(lis)
}
