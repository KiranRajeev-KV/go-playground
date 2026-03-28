package coordinator

import (
	"context"
	"fmt"
	"time"

	"distributed-transactions/internal/db"
	"distributed-transactions/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CoordinatorServer implements the TransactionCoordinator gRPC service.
type CoordinatorServer struct {
	pb.UnimplementedTransactionCoordinatorServer
	service *CoordinatorService
}

// NewCoordinatorServer creates a new coordinator server.
func NewCoordinatorServer(service *CoordinatorService) *CoordinatorServer {
	return &CoordinatorServer{
		service: service,
	}
}

// SubmitOrder handles order submission and executes the 2PC protocol.
func (s *CoordinatorServer) SubmitOrder(ctx context.Context, req *pb.SubmitOrderRequest) (*pb.SubmitOrderResponse, error) {
	transactionID := fmt.Sprintf("tx-%d-%s", time.Now().UnixNano(), req.UserId)

	tx := &CoordinatorTransaction{
		TransactionID: transactionID,
		OrderDetails: OrderDetails{
			UserID:          req.UserId,
			ItemID:          req.ItemId,
			Quantity:        req.Quantity,
			Amount:          req.Amount,
			ShippingAddress: req.ShippingAddress,
		},
		State:     StateInit,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save to database for recovery tracking
	coordTx := &db.CoordinatorTransaction{
		TransactionID:   transactionID,
		State:           db.StatePending,
		UserID:          req.UserId,
		ItemID:          req.ItemId,
		Quantity:        req.Quantity,
		Amount:          req.Amount,
		ShippingAddress: req.ShippingAddress,
	}
	_ = s.service.db.SaveCoordinatorTransaction(ctx, nil, coordTx)

	s.service.store.Create(tx)

	state, err := s.service.ExecuteTwoPhaseCommit(ctx, tx)

	// Update state in database
	if state == StateCommitted {
		_ = s.service.db.UpdateCoordinatorTransactionState(ctx, transactionID, db.StateCommitted)
	} else {
		_ = s.service.db.UpdateCoordinatorTransactionState(ctx, transactionID, db.StateAborted)
	}

	if err != nil {
		return &pb.SubmitOrderResponse{
			TransactionId: transactionID,
			Status:        string(StateAborted),
		}, nil
	}

	return &pb.SubmitOrderResponse{
		TransactionId: transactionID,
		Status:        string(state),
	}, nil
}

// GetOrderStatus returns the current status of a transaction.
func (s *CoordinatorServer) GetOrderStatus(ctx context.Context, req *pb.OrderStatusRequest) (*pb.OrderStatusResponse, error) {
	tx, ok := s.service.store.Get(req.TransactionId)
	if !ok {
		return &pb.OrderStatusResponse{
			TransactionId: req.TransactionId,
			Status:        "NOT_FOUND",
		}, nil
	}

	return &pb.OrderStatusResponse{
		TransactionId: tx.TransactionID,
		Status:        string(tx.GetState()),
	}, nil
}

// CreateParticipantClients creates gRPC clients for all participant services.
func CreateParticipantClients(inventoryAddr, paymentAddr, shippingAddr string) (map[string]pb.TransactionParticipantClient, error) {
	clients := make(map[string]pb.TransactionParticipantClient)

	connInv, err := grpc.NewClient(inventoryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to inventory: %w", err)
	}
	clients["inventory"] = pb.NewTransactionParticipantClient(connInv)

	connPay, err := grpc.NewClient(paymentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to payment: %w", err)
	}
	clients["payment"] = pb.NewTransactionParticipantClient(connPay)

	connShip, err := grpc.NewClient(shippingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to shipping: %w", err)
	}
	clients["shipping"] = pb.NewTransactionParticipantClient(connShip)

	return clients, nil
}
