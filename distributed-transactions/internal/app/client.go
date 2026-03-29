package app

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"distributed-transactions/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	reqSent      atomic.Int64
	reqCommitted atomic.Int64
	reqAborted   atomic.Int64
	reqErrors    atomic.Int64
)

type clientConfig struct {
	coordinatorAddr string
	tps             int
	duration        time.Duration
	warmup          time.Duration
}

var users = []string{"user-001", "user-002", "user-003"}
var items = []string{"laptop-001", "phone-001", "tablet-001"}

func RunClient(coordinatorAddr string, tps int, duration, warmup time.Duration) error {
	cfg := clientConfig{
		coordinatorAddr: coordinatorAddr,
		tps:             tps,
		duration:        duration,
		warmup:          warmup,
	}

	resetCounters()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.duration+cfg.warmup+5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(cfg.coordinatorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to coordinator: %w", err)
	}
	defer conn.Close()

	client := pb.NewTransactionCoordinatorClient(conn)

	go scoreboardLoop(ctx)

	if err := runLoadTest(ctx, client, cfg); err != nil {
		return err
	}

	printSummary(cfg, client)
	return nil
}

func resetCounters() {
	reqSent.Store(0)
	reqCommitted.Store(0)
	reqAborted.Store(0)
	reqErrors.Store(0)
}

func scoreboardLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sent := reqSent.Load()
			committed := reqCommitted.Load()
			aborted := reqAborted.Load()
			errors := reqErrors.Load()
			inFlight := sent - committed - aborted - errors

			fmt.Printf("\r[Metrics] Sent: %-6d | In-Flight: %-4d | Committed: %-6d | Aborted: %-6d | Errors: %-4d",
				sent, inFlight, committed, aborted, errors)
		}
	}
}

func runLoadTest(ctx context.Context, client pb.TransactionCoordinatorClient, cfg clientConfig) error {
	totalDuration := cfg.duration + cfg.warmup
	runCtx, cancel := context.WithTimeout(ctx, totalDuration)
	defer cancel()

	ticker := time.NewTicker(calculateInterval(cfg.tps))
	defer ticker.Stop()

	startTime := time.Now()
	step := 0

	for {
		select {
		case <-runCtx.Done():
			return nil
		case <-ticker.C:
			elapsed := time.Since(startTime)
			effectiveTPS := calculateEffectiveTPSFromTime(elapsed, cfg)
			ticker.Reset(calculateInterval(effectiveTPS))

			go sendRequest(runCtx, client, cfg)
			step++
		}
	}
}

func calculateEffectiveTPS(step int, cfg clientConfig) int {
	return calculateEffectiveTPSFromTime(time.Duration(step)*100*time.Millisecond, cfg)
}

func calculateEffectiveTPSFromTime(elapsed time.Duration, cfg clientConfig) int {
	if elapsed <= 0 {
		return 1
	}

	if cfg.tps <= 0 {
		return 1
	}

	if cfg.warmup == 0 {
		return cfg.tps
	}

	progress := float64(elapsed) / float64(cfg.warmup)
	if progress >= 1.0 {
		return cfg.tps
	}

	segments := 4
	quarter := cfg.tps / 4
	if quarter < 1 {
		quarter = 1
	}
	targets := []int{1, quarter, cfg.tps / 2, cfg.tps * 3 / 4, cfg.tps}
	segmentIndex := int(progress * float64(segments))
	if segmentIndex >= len(targets)-1 {
		return targets[len(targets)-1]
	}

	segmentStart := targets[segmentIndex]
	segmentEnd := targets[segmentIndex+1]
	segmentProgress := (progress*float64(segments) - float64(segmentIndex))

	return segmentStart + int(float64(segmentEnd-segmentStart)*segmentProgress)
}

func calculateInterval(tps int) time.Duration {
	if tps <= 0 {
		return time.Second
	}
	return time.Duration(1e9/tps) * time.Nanosecond
}

func sendRequest(ctx context.Context, client pb.TransactionCoordinatorClient, cfg clientConfig) {
	reqSent.Add(1)

	userID := users[rand.Intn(len(users))]
	itemID := items[rand.Intn(len(items))]
	quantity := int32(rand.Intn(3) + 1)

	prices := map[string]float64{
		"laptop-001": 1999.99,
		"phone-001":  999.99,
		"tablet-001": 1099.99,
	}
	amount := prices[itemID] * float64(quantity)

	req := &pb.SubmitOrderRequest{
		UserId:         userID,
		ItemId:         itemID,
		Quantity:       quantity,
		Amount:         amount,
		IdempotencyKey: fmt.Sprintf("key-%d", time.Now().UnixNano()),
	}

	resp, err := client.SubmitOrder(ctx, req)

	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() != codes.OK {
			reqErrors.Add(1)
		}
		return
	}

	switch resp.Status {
	case "COMMITTED":
		reqCommitted.Add(1)
	case "ABORTED":
		reqAborted.Add(1)
	default:
		reqErrors.Add(1)
	}
}

func printSummary(cfg clientConfig, client pb.TransactionCoordinatorClient) {
	sent := reqSent.Load()
	committed := reqCommitted.Load()
	aborted := reqAborted.Load()
	errCount := reqErrors.Load()
	total := committed + aborted + errCount

	fmt.Println()
	fmt.Println()
	fmt.Println("========== BENCHMARK SUMMARY ==========")
	fmt.Printf("Duration:    %v\n", cfg.duration)
	fmt.Printf("Target TPS:  %d\n", cfg.tps)
	fmt.Printf("Warmup:      %v\n", cfg.warmup)
	fmt.Println()
	fmt.Printf("Total Sent:      %d\n", sent)
	fmt.Printf("Committed:      %d (%.1f%%)\n", committed, percentage(committed, total))
	fmt.Printf("Aborted:         %d (%.1f%%)\n", aborted, percentage(aborted, total))
	fmt.Printf("Errors:          %d (%.1f%%)\n", errCount, percentage(errCount, total))
	fmt.Println()

	if sent > 0 {
		actualTPS := float64(total) / cfg.duration.Seconds()
		fmt.Printf("Actual TPS:      %.2f\n", actualTPS)
	}

	if client != nil {
		stats, err := client.GetChaosStats(context.Background(), &pb.ChaosStatsRequest{})
		if err == nil {
			totalChaos := stats.Drops + stats.Delays + stats.Phantoms
			fmt.Println()
			fmt.Println("--- Chaos Injection Stats ---")
			fmt.Printf("Drops:    %d\n", stats.Drops)
			fmt.Printf("Delays:   %d\n", stats.Delays)
			fmt.Printf("Phantoms: %d\n", stats.Phantoms)
			fmt.Printf("Total:    %d\n", totalChaos)
		}
	}

	fmt.Println("========================================")
}

func percentage(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
