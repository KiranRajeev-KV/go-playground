package middleware

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const chaosHeader = "x-chaos-simulate"

type ChaosAction string

const (
	ChaosActionDrop    ChaosAction = "drop"
	ChaosActionDelay   ChaosAction = "delay"
	ChaosActionPhantom ChaosAction = "phantom"
	ChaosActionLog     ChaosAction = "log"
)

type ChaosConfig struct {
	TargetService string
	Action        ChaosAction
	Phase         string
	Probability   int
	DelayDuration time.Duration
}

type ChaosInterceptor struct {
	participantType string
	delayDuration   time.Duration
}

func NewChaosInterceptor(participantType string, delayDuration time.Duration) *ChaosInterceptor {
	return &ChaosInterceptor{
		participantType: participantType,
		delayDuration:   delayDuration,
	}
}

func ParseChaosHeader(header string, delayDuration time.Duration) (*ChaosConfig, error) {
	if header == "" || header == "none" {
		return nil, nil
	}

	var target, actionStr, phase, probStr string
	var probability int = 100

	parts := strings.Split(header, ":")
	flag := parts[0]
	if len(parts) > 1 {
		probStr = parts[1]
	}

	flagParts := strings.Split(flag, "-")
	if len(flagParts) != 3 {
		return nil, fmt.Errorf("invalid chaos format: %s (expected target-action-phase:probability)", header)
	}
	target = flagParts[0]
	actionStr = flagParts[1]
	phase = flagParts[2]

	action := ChaosAction(actionStr)
	if action != ChaosActionDrop && action != ChaosActionDelay && action != ChaosActionPhantom && action != ChaosActionLog {
		return nil, fmt.Errorf("invalid chaos action: %s (expected drop, delay, phantom, or log)", actionStr)
	}

	if probStr != "" {
		var err error
		probability, err = strconv.Atoi(probStr)
		if err != nil || probability < 1 || probability > 100 {
			return nil, fmt.Errorf("invalid probability: %s (expected 1-100)", probStr)
		}
	}

	return &ChaosConfig{
		TargetService: target,
		Action:        action,
		Phase:         phase,
		Probability:   probability,
		DelayDuration: delayDuration,
	}, nil
}

func (i *ChaosInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return handler(ctx, req)
		}

		chaosValues := md.Get(chaosHeader)
		if len(chaosValues) == 0 {
			return handler(ctx, req)
		}

		chaosStr := chaosValues[0]
		if chaosStr == "" || chaosStr == "none" {
			return handler(ctx, req)
		}

		config, err := ParseChaosHeader(chaosStr, i.delayDuration)
		if err != nil {
			log.Printf("[Chaos] Failed to parse chaos header: %v", err)
			return handler(ctx, req)
		}

		phase := i.mapMethodToPhase(info.FullMethod)
		if phase == "" {
			return handler(ctx, req)
		}

		if config.Phase != phase {
			return handler(ctx, req)
		}

		if config.TargetService != i.participantType && config.TargetService != "all" {
			log.Printf("[Chaos] Target mismatch: chaos targets %s, but this is %s - skipping", config.TargetService, i.participantType)
			return handler(ctx, req)
		}

		log.Printf("[Chaos] Applying %s action to %s at phase %s (target: %s)", config.Action, i.participantType, phase, config.TargetService)

		switch config.Action {
		case ChaosActionDrop:
			log.Printf("[Chaos] DROP: Silently dropping request for %s at phase %s", i.participantType, phase)
			return nil, status.Error(codes.Unavailable, "service unavailable")

		case ChaosActionDelay:
			log.Printf("[Chaos] DELAY: Adding %v delay for %s at phase %s", config.DelayDuration, i.participantType, phase)
			time.Sleep(config.DelayDuration)
			return handler(ctx, req)

		case ChaosActionPhantom:
			log.Printf("[Chaos] PHANTOM: Processing request but dropping response for %s at phase %s", i.participantType, phase)
			_, err := handler(ctx, req)
			if err != nil {
				return nil, err
			}
			return nil, status.Error(codes.Unavailable, "connection lost")

		case ChaosActionLog:
			log.Printf("[Chaos] LOG: Chaos instruction received for %s at phase %s (no action taken)", i.participantType, phase)
			return handler(ctx, req)
		}

		return handler(ctx, req)
	}
}

func (i *ChaosInterceptor) mapMethodToPhase(fullMethod string) string {
	switch fullMethod {
	case "/pb.TransactionParticipant/Prepare":
		return "prepare"
	case "/pb.TransactionParticipant/Commit":
		return "commit"
	case "/pb.TransactionParticipant/Abort":
		return "abort"
	case "/pb.TransactionParticipant/CanCommit":
		return "cancommit"
	case "/pb.TransactionParticipant/PreCommit":
		return "precommit"
	case "/pb.TransactionParticipant/DoCommit":
		return "docommit"
	default:
		return ""
	}
}

func WithChaos(ctx context.Context, chaos string) context.Context {
	if chaos == "" || chaos == "none" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, chaosHeader, chaos)
}
