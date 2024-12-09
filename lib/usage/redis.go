package usage

import (
	"github.com/go-redis/redis/v8"
	"context"
	"sync"
	"sync/atomic"
)

// PubSubClient abstracts Pub/Sub operations for flexibility in testing.
type PubSubClient interface {
	Subscribe(ctx context.Context, channel string) <-chan string
	Publish(ctx context.Context, channel, message string)
}

// RedisPubSubClient implements PubSubClient using a Redis backend.
type RedisPubSubClient struct {
	client *redis.Client
}

func (rpc *RedisPubSubClient) Subscribe(ctx context.Context, channel string) <-chan string {
	pubsub := rpc.client.Subscribe(ctx, channel)
	ch := make(chan string)
	go func() {
		defer close(ch)
		for msg := range pubsub.Channel() {
			ch <- msg.Payload
		}
	}()
	return ch
}

func (rpc *RedisPubSubClient) Publish(ctx context.Context, channel, message string) {
	rpc.client.Publish(ctx, channel, message)
}

// NotificationManager manages Pub/Sub subscriptions and dispatches notifications.
type NotificationManager struct {
	client   PubSubClient
	trackers map[string]*redisUsageTracker
	mutex    sync.Mutex
}

// NewNotificationManager creates a new NotificationManager.
func NewNotificationManager(client PubSubClient) *NotificationManager {
	manager := &NotificationManager{
		client:   client,
		trackers: make(map[string]*redisUsageTracker),
	}
	go manager.startSubscriber("api_key_usage_notifications")
	return manager
}

// RegisterTracker registers a redisUsageTracker with the manager.
func (nm *NotificationManager) RegisterTracker(key string, tracker *redisUsageTracker) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()
	nm.trackers[key] = tracker
}

// startSubscriber listens for notifications and updates the appropriate tracker.
func (nm *NotificationManager) startSubscriber(channel string) {
	ch := nm.client.Subscribe(context.Background(), channel)
	for msg := range ch {
		nm.mutex.Lock()
		tracker, exists := nm.trackers[msg]
		nm.mutex.Unlock()
		if exists {
			tracker.Error.Store(ErrRequestLimit)
		}
	}
}

// redisUsageTracker tracks usage of an API key in Redis.
type redisUsageTracker struct {
	BatchSize int64
	Uses      *int64
	Key       string
	Client    *redis.Client
	PubSub    PubSubClient
	Error     atomic.Value // Stores an error state when Redis usage drops below zero.
}

// Use checks whether an auth token is available for use, decrementing counters if appropriate, and
// returning errors if the token is no longer available.
func (ut *redisUsageTracker) Use() error {
	// Check if the tracker is in an error state.
	err := ut.Error.Load()
	if err != nil {
		return err.(error)
	}
	
	localUses := atomic.AddInt64(ut.Uses, 1)
	
	if localUses >= ut.BatchSize {
		// Reset local usage counter atomically to 0 and decrement Redis by localUses.
		if atomic.CompareAndSwapInt64(ut.Uses, localUses, 0) {
			ctx := context.Background()
			remaining, err := ut.Client.DecrBy(ctx, ut.Key, localUses).Result()
			if err != nil {
				// Rollback the local count on failure.
				atomic.AddInt64(ut.Uses, localUses)
				return err
			}
			// If Redis usage drops below zero, set the tracker to an error state and publish a notification.
			if remaining < 0 {
				ut.Error.Store(ErrRequestLimit)
				ut.PubSub.Publish(ctx, "api_key_usage_notifications", ut.Key)
				return ut.Error.Load().(error)
			}
		}
	}
	return nil
}