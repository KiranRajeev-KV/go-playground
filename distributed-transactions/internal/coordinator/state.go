package coordinator

import (
	"context"
	"errors"
	"sync"
	"time"

	"distributed-transactions/internal/db"
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

	Participants []string
}

// NewCoordinatorService creates a new coordinator service.
func NewCoordinatorService(database *db.DB, clients map[string]pb.TransactionParticipantClient) *CoordinatorService {
	return &CoordinatorService{
		db:                 database,
		store:              NewTransactionStore(),
		participantClients: clients,
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
	s.store.UpdateState(tx.TransactionID, StatePreparing)

	results, err := s.sendPrepareToAll(ctx, tx)
	if err != nil {
		s.store.UpdateState(tx.TransactionID, StateAborted)
		s.sendAbortToAll(tx.TransactionID)
		return StateAborted, err
	}

	for _, result := range results {
		tx.Votes = append(tx.Votes, ParticipantVote(result))
	}

	for _, result := range results {
		if !result.VoteYes {
			s.store.UpdateState(tx.TransactionID, StateAborted)
			s.sendAbortToAll(tx.TransactionID)
			return StateAborted, nil
		}
	}

	s.store.UpdateState(tx.TransactionID, StateCommitting)

	err = s.sendCommitToAll(ctx, tx.TransactionID)
	if err != nil {
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
	client, ok := s.participantClients[participant]
	if !ok {
		return PrepareResult{Participant: participant, VoteYes: false, Reason: "no client"}, errors.New("no client for participant")
	}

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
			_, err := client.Commit(ctx, &pb.CommitRequest{TransactionId: transactionID})
			return err
		})
	}

	return g.Wait()
}

// sendAbortToAll sends abort requests to all participants.
func (s *CoordinatorService) sendAbortToAll(transactionID string) {
	g, _ := errgroup.WithContext(context.Background())

	for _, participant := range s.Participants {
		g.Go(func() error {
			client, ok := s.participantClients[participant]
			if !ok {
				return nil
			}
			_, err := client.Abort(context.Background(), &pb.AbortRequest{TransactionId: transactionID})
			return err
		})
	}

	_ = g.Wait()
}
