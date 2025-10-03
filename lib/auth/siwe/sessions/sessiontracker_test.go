package sessiontracker

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/philippgille/gokv/syncmap"
)

func newTestTracker() *Tracker {
	store := syncmap.NewStore(syncmap.DefaultOptions)
	lm := NewInMemoryLockManager()
	return NewTracker(store, lm)
}

func TestAddAndGetCollectionQuantity(t *testing.T) {
	tracker := newTestTracker()
	ctx, _ :=  context.WithTimeoutCause(context.Background(), time.Second * 10, errors.New("timeout"))

	if err := tracker.AddSession(ctx, "s1", "c1", big.NewInt(5), time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("failed to add session: %v", err)
	}
	if err := tracker.AddSession(ctx, "s2", "c1", big.NewInt(10), time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("failed to add session: %v", err)
	}

	total, err := tracker.GetCollectionQuantity(ctx, "c1")
	if err != nil {
		t.Fatalf("failed to get Quantity: %v", err)
	}
	if total.Cmp(big.NewInt(15)) != 0 {
		t.Errorf("expected total 15, got %d", total)
	}
}

func TestRevokeSession(t *testing.T) {
	tracker := newTestTracker()
	ctx, _ :=  context.WithTimeoutCause(context.Background(), time.Second * 10, errors.New("timeout"))

	_ = tracker.AddSession(ctx, "s1", "c1", big.NewInt(5), time.Now().Add(time.Minute))
	_ = tracker.AddSession(ctx, "s2", "c1", big.NewInt(10), time.Now().Add(time.Minute))

	if err := tracker.RevokeSession(ctx, "c1", "s1"); err != nil {
		t.Fatalf("failed to revoke session: %v", err)
	}

	total, _ := tracker.GetCollectionQuantity(ctx, "c1")
	if total.Cmp(big.NewInt(10)) != 0 {
		t.Errorf("expected total 10 after revoke, got %d", total)
	}
}

func TestExpiredSessionCleanup(t *testing.T) {
	tracker := newTestTracker()
	ctx, _ :=  context.WithTimeoutCause(context.Background(), time.Second * 10, errors.New("timeout"))

	_ = tracker.AddSession(ctx, "s1", "c1", big.NewInt(5), time.Now().Add(-time.Minute))
	_ = tracker.AddSession(ctx, "s2", "c1", big.NewInt(10), time.Now().Add(time.Minute))

	total, _ := tracker.GetCollectionQuantity(ctx, "c1")
	if total.Cmp(big.NewInt(10)) != 0 {
		t.Errorf("expected total 10 after expired cleanup, got %d", total)
	}
}

func TestMultipleCollections(t *testing.T) {
	tracker := newTestTracker()
	ctx, _ :=  context.WithTimeoutCause(context.Background(), time.Second * 10, errors.New("timeout"))

	_ = tracker.AddSession(ctx, "a1", "c1", big.NewInt(5), time.Now().Add(time.Minute))
	_ = tracker.AddSession(ctx, "a2", "c2", big.NewInt(7), time.Now().Add(time.Minute))
	_ = tracker.AddSession(ctx, "a3", "c2", big.NewInt(3), time.Now().Add(time.Minute))

	total1, _ := tracker.GetCollectionQuantity(ctx, "c1")
	total2, _ := tracker.GetCollectionQuantity(ctx, "c2")

	if total1.Cmp(big.NewInt(5)) != 0 {
		t.Errorf("expected total 5 for c1, got %d", total1)
	}
	if total2.Cmp(big.NewInt(10)) != 0 {
		t.Errorf("expected total 10 for c2, got %d", total2)
	}
}

func TestRevokeNonexistentSession(t *testing.T) {
	tracker := newTestTracker()
	ctx, _ :=  context.WithTimeoutCause(context.Background(), time.Second * 10, errors.New("timeout"))

	_ = tracker.AddSession(ctx, "a1", "c1", big.NewInt(5), time.Now().Add(time.Minute))
	err := tracker.RevokeSession(ctx, "c1", "nonexistent")
	if err == nil {
		t.Errorf("expected error revoking nonexistent session, got nil")
	}
}