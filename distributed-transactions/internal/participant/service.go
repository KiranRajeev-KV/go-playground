package participant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"distributed-transactions/internal/db"
	"distributed-transactions/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// ParticipantType identifies which participant service this is.
type ParticipantType string

const (
	ParticipantInventory ParticipantType = "inventory"
	ParticipantPayment   ParticipantType = "payment"
)

// ParticipantServer implements the TransactionParticipant gRPC service.
type ParticipantServer struct {
	pb.UnimplementedTransactionParticipantServer
	db             *db.DB
	participant    ParticipantType
	EnableRecovery bool
	CommitTimeout  time.Duration
}

// NewParticipantServer creates a new participant server.
func NewParticipantServer(database *db.DB, participant ParticipantType, recover bool, commitTimeout time.Duration) *ParticipantServer {
	return &ParticipantServer{
		db:             database,
		participant:    participant,
		EnableRecovery: recover,
		CommitTimeout:  commitTimeout,
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
		voteYes, reason = s.prepareInventory(ctx, req)
	case ParticipantPayment:
		voteYes, reason = s.preparePayment(ctx, req)
	}

	var msg proto.Message
	switch p := req.Payload.(type) {
	case *pb.PrepareRequest_Inventory:
		msg = p.Inventory
	case *pb.PrepareRequest_Payment:
		msg = p.Payment
	}
	payload, _ := proto.Marshal(msg)

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
func (s *ParticipantServer) prepareInventory(ctx context.Context, req *pb.PrepareRequest) (bool, string) {
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
func (s *ParticipantServer) preparePayment(ctx context.Context, req *pb.PrepareRequest) (bool, string) {
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
	inv := &pb.InventoryPayload{}
	if err := proto.Unmarshal([]byte(payload), inv); err != nil {
		return err
	}
	return s.db.ReserveInventory(ctx, tx, inv.ItemId, int(inv.Quantity))
}

// applyPayment charges the payment account.
func (s *ParticipantServer) applyPayment(ctx context.Context, tx *sql.Tx, payload string) error {
	pay := &pb.PaymentPayload{}
	if err := proto.Unmarshal([]byte(payload), pay); err != nil {
		return err
	}
	return s.db.ChargePayment(ctx, tx, pay.UserId, pay.Amount)
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

// Recover handles crash recovery for pending transactions.
func (s *ParticipantServer) Recover(ctx context.Context) error {
	logs, err := s.db.GetPreparedTransactions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get prepared transactions: %w", err)
	}

	fmt.Printf("[%s] Found %d prepared transactions to check\n", s.participant, len(logs))

	timeout := s.CommitTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	for _, log := range logs {
		fmt.Printf("[%s] Checking transaction %s (updated: %s)\n", s.participant, log.TransactionID, log.UpdatedAt)

		// Parse the updated_at time and check if timeout has elapsed
		// For simplicity, we'll check if the updated_at is older than timeout
		// SQLite datetime format: YYYY-MM-DD HH:MM:SS
		layout := "2006-01-02 15:04:05"
		updatedTime, err := time.Parse(layout, log.UpdatedAt)
		if err != nil {
			fmt.Printf("[%s] Failed to parse time for %s: %v\n", s.participant, log.TransactionID, err)
			continue
		}

		elapsed := time.Since(updatedTime)
		if elapsed > timeout {
			fmt.Printf("[%s] Transaction %s timed out after %v, auto-aborting\n", s.participant, log.TransactionID, elapsed)
			_, _ = s.db.DB.ExecContext(ctx,
				`UPDATE transaction_log SET state = ?, updated_at = datetime('now') WHERE transaction_id = ? AND participant = ?`,
				db.StateAborted, log.TransactionID, s.participant)
		} else {
			fmt.Printf("[%s] Transaction %s still within timeout (%v remaining), waiting for coordinator\n", s.participant, log.TransactionID, timeout-elapsed)
		}
	}

	return nil
}

// GetStatus returns the current state of a transaction.
func (s *ParticipantServer) GetStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	log, err := s.db.GetTransactionLog(ctx, req.TransactionId, string(s.participant))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return &pb.StatusResponse{
				TransactionId: req.TransactionId,
				State:         "NOT_FOUND",
			}, nil
		}
		return nil, err
	}

	return &pb.StatusResponse{
		TransactionId: log.TransactionID,
		State:         string(log.State),
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
