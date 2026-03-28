package participant

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"distributed-transactions/internal/db"
	"distributed-transactions/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ParticipantType identifies which participant service this is.
type ParticipantType string

const (
	ParticipantInventory ParticipantType = "inventory"
	ParticipantPayment   ParticipantType = "payment"
	ParticipantShipping  ParticipantType = "shipping"
)

// ParticipantServer implements the TransactionParticipant gRPC service.
type ParticipantServer struct {
	pb.UnimplementedTransactionParticipantServer
	db          *db.DB
	participant ParticipantType
}

// NewParticipantServer creates a new participant server.
func NewParticipantServer(database *db.DB, participant ParticipantType) *ParticipantServer {
	return &ParticipantServer{
		db:          database,
		participant: participant,
	}
}

// Prepare handles the prepare phase of 2PC.
// It validates business logic and records the transaction in the WAL.
func (s *ParticipantServer) Prepare(ctx context.Context, req *pb.PrepareRequest) (*pb.PrepareResponse, error) {
	tx, err := s.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return &pb.PrepareResponse{
			TransactionId: req.TransactionId,
			VoteYes:       false,
			Reason:        err.Error(),
		}, nil
	}
	defer func() { _ = tx.Rollback() }()

	var voteYes bool
	var reason string

	switch s.participant {
	case ParticipantInventory:
		voteYes, reason = s.prepareInventory(ctx, tx, req)
	case ParticipantPayment:
		voteYes, reason = s.preparePayment(ctx, tx, req)
	case ParticipantShipping:
		voteYes, reason = s.prepareShipping(ctx, req)
	}

	payload, _ := json.Marshal(req.Payload)

	log := &db.TransactionLog{
		TransactionID: req.TransactionId,
		Participant:   string(s.participant),
		State:         db.StatePrepared,
		Payload:       string(payload),
	}

	if voteYes {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO transaction_log (transaction_id, participant, state, payload) VALUES (?, ?, ?, ?)`,
			log.TransactionID, log.Participant, log.State, log.Payload)
		if err != nil {
			return &pb.PrepareResponse{
				TransactionId: req.TransactionId,
				VoteYes:       false,
				Reason:        err.Error(),
			}, nil
		}
	}

	if err := tx.Commit(); err != nil {
		return &pb.PrepareResponse{
			TransactionId: req.TransactionId,
			VoteYes:       false,
			Reason:        err.Error(),
		}, nil
	}

	return &pb.PrepareResponse{
		TransactionId: req.TransactionId,
		VoteYes:       voteYes,
		Reason:        reason,
	}, nil
}

// prepareInventory validates that sufficient inventory exists for the order.
func (s *ParticipantServer) prepareInventory(ctx context.Context, _ *sql.Tx, req *pb.PrepareRequest) (bool, string) {
	inv, ok := req.Payload.(*pb.PrepareRequest_Inventory)
	if !ok {
		return false, "invalid payload type for inventory"
	}

	item, err := s.db.GetInventory(ctx, inv.Inventory.ItemId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return false, "item not found"
		}
		return false, err.Error()
	}

	if item.Quantity < int(inv.Inventory.Quantity) {
		return false, fmt.Sprintf("insufficient inventory: have %d, need %d", item.Quantity, inv.Inventory.Quantity)
	}

	return true, ""
}

// preparePayment validates that the payment account has sufficient balance.
func (s *ParticipantServer) preparePayment(ctx context.Context, _ *sql.Tx, req *pb.PrepareRequest) (bool, string) {
	pay, ok := req.Payload.(*pb.PrepareRequest_Payment)
	if !ok {
		return false, "invalid payload type for payment"
	}

	acc, err := s.db.GetPaymentAccount(ctx, pay.Payment.UserId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return false, "payment account not found"
		}
		return false, err.Error()
	}

	if acc.Balance < pay.Payment.Amount {
		return false, fmt.Sprintf("insufficient balance: have %.2f, need %.2f", acc.Balance, pay.Payment.Amount)
	}

	return true, ""
}

// prepareShipping validates that a shipping address exists for the user.
func (s *ParticipantServer) prepareShipping(ctx context.Context, req *pb.PrepareRequest) (bool, string) {
	ship, ok := req.Payload.(*pb.PrepareRequest_Shipping)
	if !ok {
		return false, "invalid payload type for shipping"
	}

	addr, err := s.db.GetShippingAddress(ctx, ship.Shipping.UserId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return false, "shipping address not found"
		}
		return false, err.Error()
	}

	if addr.Address == "" {
		return false, "no shipping address on file"
	}

	return true, ""
}

// Commit handles the commit phase of 2PC.
// It retrieves the stored payload and applies the actual business changes.
func (s *ParticipantServer) Commit(ctx context.Context, req *pb.CommitRequest) (*pb.CommitResponse, error) {
	log, err := s.db.GetTransactionLog(ctx, req.TransactionId, string(s.participant))
	if err != nil {
		return &pb.CommitResponse{
			TransactionId: req.TransactionId,
			Success:       false,
		}, err
	}

	tx, err := s.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return &pb.CommitResponse{
			TransactionId: req.TransactionId,
			Success:       false,
		}, err
	}
	defer func() { _ = tx.Rollback() }()

	switch s.participant {
	case ParticipantInventory:
		if err := s.applyInventory(ctx, tx, log.Payload); err != nil {
			return &pb.CommitResponse{TransactionId: req.TransactionId, Success: false}, err
		}
	case ParticipantPayment:
		if err := s.applyPayment(ctx, tx, log.Payload); err != nil {
			return &pb.CommitResponse{TransactionId: req.TransactionId, Success: false}, err
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE transaction_log SET state = ?, updated_at = datetime('now') WHERE transaction_id = ? AND participant = ?`,
		db.StateCommitted, req.TransactionId, s.participant)
	if err != nil {
		return &pb.CommitResponse{
			TransactionId: req.TransactionId,
			Success:       false,
		}, err
	}

	if err := tx.Commit(); err != nil {
		return &pb.CommitResponse{
			TransactionId: req.TransactionId,
			Success:       false,
		}, err
	}

	return &pb.CommitResponse{
		TransactionId: req.TransactionId,
		Success:       true,
	}, nil
}

// applyInventory decrements the inventory quantity.
func (s *ParticipantServer) applyInventory(ctx context.Context, tx *sql.Tx, payload string) error {
	var p pb.PrepareRequest
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return err
	}
	inv, ok := p.Payload.(*pb.PrepareRequest_Inventory)
	if !ok {
		return errors.New("invalid payload")
	}
	return s.db.ReserveInventory(ctx, tx, inv.Inventory.ItemId, int(inv.Inventory.Quantity))
}

// applyPayment charges the payment account.
func (s *ParticipantServer) applyPayment(ctx context.Context, tx *sql.Tx, payload string) error {
	var p pb.PrepareRequest
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return err
	}
	pay, ok := p.Payload.(*pb.PrepareRequest_Payment)
	if !ok {
		return errors.New("invalid payload")
	}
	return s.db.ChargePayment(ctx, tx, pay.Payment.UserId, pay.Payment.Amount)
}

// Abort handles the abort phase of 2PC.
// It marks the transaction as aborted in the WAL.
func (s *ParticipantServer) Abort(ctx context.Context, req *pb.AbortRequest) (*pb.AbortResponse, error) {
	_, err := s.db.DB.ExecContext(ctx,
		`UPDATE transaction_log SET state = ?, updated_at = datetime('now') WHERE transaction_id = ? AND participant = ?`,
		db.StateAborted, req.TransactionId, s.participant)
	if err != nil {
		return &pb.AbortResponse{
			TransactionId: req.TransactionId,
			Success:       false,
		}, err
	}

	return &pb.AbortResponse{
		TransactionId: req.TransactionId,
		Success:       true,
	}, nil
}

// DialCoordinator creates a gRPC client to the coordinator service.
func DialCoordinator(addr string) (pb.TransactionCoordinatorClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return pb.NewTransactionCoordinatorClient(conn), conn, nil
}
