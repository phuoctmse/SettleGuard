package risk_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/risk"
)

type fakeVelocityLimiter struct {
	counts map[uuid.UUID]int
	err    error
}

func (f *fakeVelocityLimiter) CountRecentTransactions(_ context.Context, accountID uuid.UUID, _ time.Time) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.counts[accountID], nil
}

type fakeBlocklistChecker struct {
	blocked map[uuid.UUID]bool
	err     error
}

func (f *fakeBlocklistChecker) IsBlocked(_ context.Context, accountIDs []uuid.UUID) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	for _, id := range accountIDs {
		if f.blocked[id] {
			return true, nil
		}
	}
	return false, nil
}

func testConfig() risk.Config {
	return risk.Config{
		VelocityLimit:     5,
		VelocityWindow:    5 * time.Minute,
		MismatchThreshold: 10_000_000,
	}
}

func TestScorer_Score_PassWhenNoRuleTriggers(t *testing.T) {
	acc := uuid.New()
	scorer := risk.NewScorer(testConfig(),
		&fakeVelocityLimiter{counts: map[uuid.UUID]int{acc: 0}},
		&fakeBlocklistChecker{blocked: map[uuid.UUID]bool{}},
	)

	tx := risk.TransactionInput{ID: uuid.New(), AccountIDs: []uuid.UUID{acc}, Amount: 1_000, OccurredAt: time.Now()}
	got, err := scorer.Score(context.Background(), tx)

	require.NoError(t, err)
	assert.Equal(t, risk.DecisionPass, got.Decision)
	assert.Equal(t, 0, got.Score)
	assert.Equal(t, tx.ID, got.TransactionID)
	for _, o := range got.Outcomes {
		assert.False(t, o.Triggered, "rule %s should not trigger", o.Rule)
	}
}

func TestScorer_Score_HoldOnVelocityLimit(t *testing.T) {
	acc := uuid.New()
	cfg := testConfig()
	scorer := risk.NewScorer(cfg,
		&fakeVelocityLimiter{counts: map[uuid.UUID]int{acc: cfg.VelocityLimit + 1}},
		&fakeBlocklistChecker{blocked: map[uuid.UUID]bool{}},
	)

	tx := risk.TransactionInput{ID: uuid.New(), AccountIDs: []uuid.UUID{acc}, Amount: 1_000, OccurredAt: time.Now()}
	got, err := scorer.Score(context.Background(), tx)

	require.NoError(t, err)
	assert.Equal(t, risk.DecisionHold, got.Decision)
	assert.Equal(t, 40, got.Score)
}

func TestScorer_Score_HoldOnMismatchThreshold(t *testing.T) {
	acc := uuid.New()
	cfg := testConfig()
	scorer := risk.NewScorer(cfg,
		&fakeVelocityLimiter{counts: map[uuid.UUID]int{acc: 0}},
		&fakeBlocklistChecker{blocked: map[uuid.UUID]bool{}},
	)

	tx := risk.TransactionInput{ID: uuid.New(), AccountIDs: []uuid.UUID{acc}, Amount: cfg.MismatchThreshold + 1, OccurredAt: time.Now()}
	got, err := scorer.Score(context.Background(), tx)

	require.NoError(t, err)
	assert.Equal(t, risk.DecisionHold, got.Decision)
	assert.Equal(t, 30, got.Score)
}

func TestScorer_Score_HoldOnBlocklist(t *testing.T) {
	acc := uuid.New()
	cfg := testConfig()
	scorer := risk.NewScorer(cfg,
		&fakeVelocityLimiter{counts: map[uuid.UUID]int{acc: 0}},
		&fakeBlocklistChecker{blocked: map[uuid.UUID]bool{acc: true}},
	)

	tx := risk.TransactionInput{ID: uuid.New(), AccountIDs: []uuid.UUID{acc}, Amount: 1_000, OccurredAt: time.Now()}
	got, err := scorer.Score(context.Background(), tx)

	require.NoError(t, err)
	assert.Equal(t, risk.DecisionHold, got.Decision)
	assert.Equal(t, 100, got.Score)
}

func TestScorer_Score_MultipleRulesTrigger_ScoreAccumulatesCapped100(t *testing.T) {
	acc := uuid.New()
	cfg := testConfig()

	t.Run("accumulates without cap", func(t *testing.T) {
		scorer := risk.NewScorer(cfg,
			&fakeVelocityLimiter{counts: map[uuid.UUID]int{acc: cfg.VelocityLimit + 1}},
			&fakeBlocklistChecker{blocked: map[uuid.UUID]bool{}},
		)
		tx := risk.TransactionInput{ID: uuid.New(), AccountIDs: []uuid.UUID{acc}, Amount: cfg.MismatchThreshold + 1, OccurredAt: time.Now()}
		got, err := scorer.Score(context.Background(), tx)
		require.NoError(t, err)
		assert.Equal(t, risk.DecisionHold, got.Decision)
		assert.Equal(t, 70, got.Score) // velocity(40) + mismatch(30)
	})

	t.Run("caps at 100 when sum exceeds", func(t *testing.T) {
		scorer := risk.NewScorer(cfg,
			&fakeVelocityLimiter{counts: map[uuid.UUID]int{acc: cfg.VelocityLimit + 1}},
			&fakeBlocklistChecker{blocked: map[uuid.UUID]bool{acc: true}},
		)
		tx := risk.TransactionInput{ID: uuid.New(), AccountIDs: []uuid.UUID{acc}, Amount: 1_000, OccurredAt: time.Now()}
		got, err := scorer.Score(context.Background(), tx)
		require.NoError(t, err)
		assert.Equal(t, risk.DecisionHold, got.Decision)
		assert.Equal(t, 100, got.Score) // velocity(40) + blocklist(100) = 140, capped
	})
}

func TestScorer_Score_PropagatesVelocityLimiterError(t *testing.T) {
	acc := uuid.New()
	boom := errors.New("velocity boom")
	scorer := risk.NewScorer(testConfig(),
		&fakeVelocityLimiter{err: boom},
		&fakeBlocklistChecker{blocked: map[uuid.UUID]bool{}},
	)

	tx := risk.TransactionInput{ID: uuid.New(), AccountIDs: []uuid.UUID{acc}, Amount: 1_000, OccurredAt: time.Now()}
	_, err := scorer.Score(context.Background(), tx)

	require.Error(t, err)
	assert.ErrorContains(t, err, "velocity boom")
}

func TestScorer_Score_PropagatesBlocklistCheckerError(t *testing.T) {
	acc := uuid.New()
	boom := errors.New("blocklist boom")
	scorer := risk.NewScorer(testConfig(),
		&fakeVelocityLimiter{counts: map[uuid.UUID]int{acc: 0}},
		&fakeBlocklistChecker{err: boom},
	)

	tx := risk.TransactionInput{ID: uuid.New(), AccountIDs: []uuid.UUID{acc}, Amount: 1_000, OccurredAt: time.Now()}
	_, err := scorer.Score(context.Background(), tx)

	require.Error(t, err)
	assert.ErrorContains(t, err, "blocklist boom")
}
