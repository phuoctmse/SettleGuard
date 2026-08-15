package settlement

import (
	"context"
	"log"
	"time"
)

// Scheduler periodically batches all pending_settlement transactions via
// SettlementRepository.RunBatch. Mirrors outbox.Relay.Run's ticker shape.
type Scheduler struct {
	repo     *SettlementRepository
	interval time.Duration
}

func NewScheduler(repo *SettlementRepository, interval time.Duration) *Scheduler {
	return &Scheduler{repo: repo, interval: interval}
}

// Run calls RunBatch on a fixed interval until ctx is done, returning
// ctx.Err() at that point. A batch error is logged and the next tick
// retries -- it does not stop the loop.
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := s.repo.RunBatch(ctx); err != nil {
				log.Printf("settlement scheduler: run batch: %v", err)
			}
		}
	}
}
