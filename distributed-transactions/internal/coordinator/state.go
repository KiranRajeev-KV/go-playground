package coordinator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"distributed-transactions/internal/db"
	"distributed-transactions/internal/middleware"
	"distributed-transactions/pb"

	"golang.org/x/sync/errgroup"
)

// CoordinatorState represents the current state of a distributed transaction.
type CoordinatorState string

const (
	StateInit       CoordinatorState = "INIT"
	StatePreparing  CoordinatorState = "PREPARING"
	StateCommitting CoordinatorState = "COMMITTING"
	StateCommitted  CoordinatorState = "COMMITTED"
	StateAborted    CoordinatorState = "ABORTED"
)

var ErrInvalidStateTransition = errors.New("invalid state transition")

// OrderDetails holds the order information needed for the transaction.
type OrderDetails struct {
	UserID          string
	ItemID          string
	Quantity        int32
	Amount          float64
	ShippingAddress string
}

// ParticipantVote represents a participant's vote during the prepare phase.
type ParticipantVote struct {
	Participant string
	VoteYes     bool
	Reason      string
}

// CoordinatorTransaction holds all information about a distributed transaction.
type CoordinatorTransaction struct {
	TransactionID string
	OrderDetails  OrderDetails
	State         CoordinatorState
	Votes         []ParticipantVote
	CreatedAt     time.Time
	UpdatedAt     time.Time
	mu            sync.RWMutex
}

// TransactionStore is an in-memory store for tracking active transactions.
type TransactionStore struct {
	mu           sync.RWMutex
	transactions map[string]*CoordinatorTransaction
}

// NewTransactionStore creates a new transaction store.
func NewTransactionStore() *TransactionStore {
	return &TransactionStore{
		transactions: make(map[string]*CoordinatorTransaction),
	}
}

// Create adds a new transaction to the store.
func (s *TransactionStore) Create(tx *CoordinatorTransaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions[tx.TransactionID] = tx
}

// Get retrieves a transaction by ID.
func (s *TransactionStore) Get(transactionID string) (*CoordinatorTransaction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, ok := s.transactions[transactionID]
	return tx, ok
}

// UpdateState updates the state of a transaction.
func (s *TransactionStore) UpdateState(transactionID string, state CoordinatorState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tx, ok := s.transactions[transactionID]; ok {
		tx.State = state
		tx.UpdatedAt = time.Now()
	}
}

// SetVotes sets the participant votes for a transaction.
func (t *CoordinatorTransaction) SetVotes(votes []ParticipantVote) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Votes = votes
}

// GetState returns the current state of the transaction.
func (t *CoordinatorTransaction) GetState() CoordinatorState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.State
}

// CoordinatorService manages the 2PC protocol execution.
type CoordinatorService struct {
	pb.UnimplementedTransactionCoordinatorServer
	db                 *db.DB
	store              *TransactionStore
	participantClients map[string]pb.TransactionParticipantClient
	EnableRecovery     bool
	CommitTimeout      time.Duration

	Participants []string
}

// NewCoordinatorService creates a new coordinator service.
func NewCoordinatorService(database *db.DB, clients map[string]pb.TransactionParticipantClient, recover bool, commitTimeout time.Duration) *CoordinatorService {
	return &CoordinatorService{
		db:                 database,
		store:              NewTransactionStore(),
		participantClients: clients,
		EnableRecovery:     recover,
		CommitTimeout:      commitTimeout,
		Participants:       []string{"inventory", "payment", "shipping"},
	}
}

// PrepareResult holds the result from a participant's prepare phase.
type PrepareResult struct {
	Participant string
	VoteYes     bool
	Reason      string
}

// ExecuteTwoPhaseCommit runs the full 2PC protocol for a transaction.
func (s *CoordinatorService) ExecuteTwoPhaseCommit(ctx context.Context, tx *CoordinatorTransaction) (CoordinatorState, error) {
	fmt.Printf("[Coordinator] Starting 2PC for transaction %s\n", tx.TransactionID)

	s.store.UpdateState(tx.TransactionID, StatePreparing)

	results, err := s.sendPrepareToAll(ctx, tx)
	if err != nil {
		fmt.Printf("[Coordinator] Prepare failed with error: %v\n", err)
		s.store.UpdateState(tx.TransactionID, StateAborted)
		s.sendAbortToAll(tx.TransactionID)
		return StateAborted, err
	}

	fmt.Printf("[Coordinator] Prepare results: %+v\n", results)

	for _, result := range results {
		tx.Votes = append(tx.Votes, ParticipantVote(result))
	}

	for _, result := range results {
		if !result.VoteYes {
			fmt.Printf("[Coordinator] Vote NO from %s: %s\n", result.Participant, result.Reason)
			s.store.UpdateState(tx.TransactionID, StateAborted)
			s.sendAbortToAll(tx.TransactionID)
			return StateAborted, nil
		}
	}

	s.store.UpdateState(tx.TransactionID, StateCommitting)

	fmt.Printf("[Coordinator] Sending commit to all participants\n")
	err = s.sendCommitToAll(ctx, tx.TransactionID)
	if err != nil {
		fmt.Printf("[Coordinator] Commit failed with error: %v\n", err)
		s.store.UpdateState(tx.TransactionID, StateAborted)
		return StateAborted, err
	}

	s.store.UpdateState(tx.TransactionID, StateCommitted)
	return StateCommitted, nil
}

// sendPrepareToAll sends prepare requests to all participants in parallel.
func (s *CoordinatorService) sendPrepareToAll(ctx context.Context, tx *CoordinatorTransaction) ([]PrepareResult, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := make([]PrepareResult, len(s.Participants))

	for i, participant := range s.Participants {
		g.Go(func() error {
			result, err := s.sendPrepare(ctx, participant, tx)
			if err != nil {
				results[i] = PrepareResult{Participant: participant, VoteYes: false, Reason: err.Error()}
				return err
			}
			results[i] = result
			return nil
		})
	}

	err := g.Wait()
	return results, err
}

// sendPrepare sends a prepare request to a specific participant.
func (s *CoordinatorService) sendPrepare(ctx context.Context, participant string, tx *CoordinatorTransaction) (PrepareResult, error) {
	fmt.Printf("[Coordinator] Sending prepare to %s\n", participant)
	client, ok := s.participantClients[participant]
	if !ok {
		return PrepareResult{Participant: participant, VoteYes: false, Reason: "no client"}, errors.New("no client for participant")
	}

	messageID := fmt.Sprintf("%s-%s-prepare", tx.TransactionID, participant)
	ctx = middleware.WithMessageID(ctx, messageID)

	var req *pb.PrepareRequest
	switch participant {
	case "inventory":
		req = &pb.PrepareRequest{
			TransactionId: tx.TransactionID,
			Payload: &pb.PrepareRequest_Inventory{
				Inventory: &pb.InventoryPayload{
					ItemId:   tx.OrderDetails.ItemID,
					Quantity: tx.OrderDetails.Quantity,
				},
			},
		}
	case "payment":
		req = &pb.PrepareRequest{
			TransactionId: tx.TransactionID,
			Payload: &pb.PrepareRequest_Payment{
				Payment: &pb.PaymentPayload{
					UserId: tx.OrderDetails.UserID,
					Amount: tx.OrderDetails.Amount,
				},
			},
		}
	case "shipping":
		req = &pb.PrepareRequest{
			TransactionId: tx.TransactionID,
			Payload: &pb.PrepareRequest_Shipping{
				Shipping: &pb.ShippingPayload{
					UserId:  tx.OrderDetails.UserID,
					Address: tx.OrderDetails.ShippingAddress,
				},
			},
		}
	}

	resp, err := client.Prepare(ctx, req)
	if err != nil {
		return PrepareResult{Participant: participant, VoteYes: false, Reason: err.Error()}, err
	}

	return PrepareResult{
		Participant: participant,
		VoteYes:     resp.VoteYes,
		Reason:      resp.Reason,
	}, nil
}

// sendCommitToAll sends commit requests to all participants in parallel.
func (s *CoordinatorService) sendCommitToAll(ctx context.Context, transactionID string) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, participant := range s.Participants {
		g.Go(func() error {
			client, ok := s.participantClients[participant]
			if !ok {
				return errors.New("no client for participant")
			}
			messageID := fmt.Sprintf("%s-%s-commit", transactionID, participant)
			reqCtx := middleware.WithMessageID(ctx, messageID)
			_, err := client.Commit(reqCtx, &pb.CommitRequest{
				TransactionId: transactionID,
			})
			return err
		})
	}

	return g.Wait()
}

// sendAbortToAll sends abort requests to all participants.
func (s *CoordinatorService) sendAbortToAll(transactionID string) {
	g, ctx := errgroup.WithContext(context.Background())

	for _, participant := range s.Participants {
		g.Go(func() error {
			client, ok := s.participantClients[participant]
			if !ok {
				return nil
			}
			messageID := fmt.Sprintf("%s-%s-abort", transactionID, participant)
			reqCtx := middleware.WithMessageID(ctx, messageID)
			_, err := client.Abort(reqCtx, &pb.AbortRequest{
				TransactionId: transactionID,
			})
			return err
		})
	}

	if err := g.Wait(); err != nil {
		log.Printf("failed to abort transaction %s: %v", transactionID, err)
	}
}

// Recover handles crash recovery for pending transactions.
func (s *CoordinatorService) Recover(ctx context.Context) error {
	txs, err := s.db.GetPendingCoordinatorTransactions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending transactions: %w", err)
	}

	fmt.Printf("[Coordinator] Found %d pending transactions to recover\n", len(txs))

	for _, coordTx := range txs {
		fmt.Printf("[Coordinator] Recovering transaction %s (state: %s)\n", coordTx.TransactionID, coordTx.State)

		// Query all participants for their state
		participantStates, err := s.queryParticipantStates(ctx, coordTx.TransactionID)
		if err != nil {
			fmt.Printf("[Coordinator] Failed to query participants for %s: %v\n", coordTx.TransactionID, err)
			continue
		}

		allPrepared := true
		anyAborted := false

		for participant, state := range participantStates {
			fmt.Printf("[Coordinator] %s state: %s\n", participant, state)
			if state == "ABORTED" {
				anyAborted = true
			}
			if state != "PREPARED" {
				allPrepared = false
			}
		}

		if anyAborted {
			// Send abort to all who are prepared
			fmt.Printf("[Coordinator] Aborting transaction %s\n", coordTx.TransactionID)
			s.sendAbortToAll(coordTx.TransactionID)
			_ = s.db.UpdateCoordinatorTransactionState(ctx, coordTx.TransactionID, db.StateAborted)
		} else if allPrepared {
			// All prepared, retry commit
			fmt.Printf("[Coordinator] Retrying commit for %s\n", coordTx.TransactionID)
			err := s.sendCommitToAll(ctx, coordTx.TransactionID)
			if err != nil {
				fmt.Printf("[Coordinator] Commit failed: %v\n", err)
				_ = s.db.UpdateCoordinatorTransactionState(ctx, coordTx.TransactionID, db.StateAborted)
			} else {
				_ = s.db.UpdateCoordinatorTransactionState(ctx, coordTx.TransactionID, db.StateCommitted)
			}
		} else {
			// Incomplete prepare, abort
			fmt.Printf("[Coordinator] Incomplete prepare, aborting %s\n", coordTx.TransactionID)
			s.sendAbortToAll(coordTx.TransactionID)
			_ = s.db.UpdateCoordinatorTransactionState(ctx, coordTx.TransactionID, db.StateAborted)
		}
	}

	return nil
}

// queryParticipantStates queries all participants for the state of a transaction.
func (s *CoordinatorService) queryParticipantStates(ctx context.Context, transactionID string) (map[string]string, error) {
	states := make(map[string]string)

	for _, participant := range s.Participants {
		client, ok := s.participantClients[participant]
		if !ok {
			states[participant] = "UNKNOWN"
			continue
		}

		resp, err := client.GetStatus(ctx, &pb.StatusRequest{TransactionId: transactionID})
		if err != nil {
			fmt.Printf("[Coordinator] Failed to get status from %s: %v\n", participant, err)
			states[participant] = "UNKNOWN"
			continue
		}

		states[participant] = resp.State
		fmt.Printf("[Coordinator] %s reported state: %s\n", participant, resp.State)
	}

	return states, nil
}
