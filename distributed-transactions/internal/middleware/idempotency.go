package middleware

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"distributed-transactions/internal/db"
	"distributed-transactions/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const messageIDKey = "x-message-id"

var ErrNotFound = errors.New("not found")

type IdempotencyStore struct {
	db  *sql.DB
	ttl time.Duration
}

func NewIdempotencyStore(database *db.DB, ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{db: database.DB, ttl: ttl}
}

func (s *IdempotencyStore) Get(ctx context.Context, messageID string, response proto.Message) (bool, error) {
	var responseData []byte
	var expiresAt int64
	err := s.db.QueryRowContext(ctx,
		"SELECT response, expires_at FROM idempotency_keys WHERE message_id = ?",
		messageID,
	).Scan(&responseData, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	if time.Now().Unix() > expiresAt {
		_, _ = s.db.ExecContext(ctx, "DELETE FROM idempotency_keys WHERE message_id = ?", messageID)
		return false, nil
	}

	if err := proto.Unmarshal(responseData, response); err != nil {
		return false, err
	}

	return true, nil
}

func (s *IdempotencyStore) Put(ctx context.Context, messageID string, response proto.Message) error {
	responseData, err := proto.Marshal(response)
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(s.ttl).Unix()

	_, err = s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO idempotency_keys (message_id, response, expires_at) VALUES (?, ?, ?)",
		messageID, responseData, expiresAt,
	)
	return err
}

type IdempotencyInterceptor struct {
	store *IdempotencyStore
	ttl   time.Duration
}

func NewIdempotencyInterceptor(store *IdempotencyStore, ttl time.Duration) *IdempotencyInterceptor {
	return &IdempotencyInterceptor{store: store, ttl: ttl}
}

func (i *IdempotencyInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		messageID := getMessageID(ctx)
		if messageID == "" {
			return handler(ctx, req)
		}

		var resp proto.Message
		switch info.FullMethod {
		case "/pb.TransactionParticipant/Prepare":
			resp = &pb.PrepareResponse{}
		case "/pb.TransactionParticipant/Commit":
			resp = &pb.CommitResponse{}
		case "/pb.TransactionParticipant/Abort":
			resp = &pb.AbortResponse{}
		case "/pb.TransactionParticipant/GetStatus":
			resp = &pb.StatusResponse{}
		default:
			return handler(ctx, req)
		}

		cached, err := i.store.Get(ctx, messageID, resp)
		if err != nil {
			return nil, err
		}

		if cached {
			return resp, nil
		}

		respIntf, err := handler(ctx, req)

		if err == nil {
			if respProto, ok := respIntf.(proto.Message); ok {
				_ = i.store.Put(ctx, messageID, respProto)
			}
		}

		return respIntf, err
	}
}

func getMessageID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	ids := md.Get(messageIDKey)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func WithMessageID(ctx context.Context, messageID string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, messageIDKey, messageID)
}
