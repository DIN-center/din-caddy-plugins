package usage

import (
	"github.com/go-redis/redis/v8"
	"context"
	"sync"
	"sync/atomic"
	"time"
	"fmt"
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

var (
	nmSetup sync.Once
	nm *NotificationManager
)

func NewRedisUsageTracker(batchSize, maxUses int64, key string, expiration time.Time, client *redis.Client) UsageTracker {
	nmSetup.Do(func() {
		nm = NewNotificationManager(&RedisPubSubClient{client})
		go nm.startSubscriber("api_key_usage_notifications")
	})
	client.SetNX(context.Background(), key, maxUses, time.Until(expiration))
	rt := &redisUsageTracker{
		BatchSize: batchSize,
		Uses: new(int64),
		Key: key,
		Client: client,
		PubSub: nm.client,
	}
	nm.RegisterTracker(key, rt)
	return rt
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
			if remaining <= 0 {
				ut.Error.Store(ErrRequestLimit)
				ut.PubSub.Publish(ctx, "api_key_usage_notifications", ut.Key)
				if remaining < 0 {
					return ut.Error.Load().(error)
				}
			}
		}
	}
	return nil
}

func NewRedisTrackerManager(client *redis.Client, batchSize int64) TrackerManager {
	return &redisUsageTrackerManager{
		m: make(map[string]UsageTracker),
		c: client,
		bs: batchSize,
		ck: "din_caddy_counter_key",
	}
}

type redisUsageTrackerManager struct {
	m    map[string]UsageTracker
	c    *redis.Client
	bs   int64
	lock sync.RWMutex
	ck   string
}

func (m *redisUsageTrackerManager) Create(uses int64, exp time.Time) (string, error) {
	res := m.c.Incr(context.Background(), m.ck)
	if err := res.Err(); err != nil {
		return "", err
	}
	var key string
	if keyInt, err := res.Result(); err != nil {
		return "", err
	} else {
		key = fmt.Sprintf("%v-%v", m.ck, keyInt)
	}
	m.lock.Lock()
	tracker := NewRedisUsageTracker(m.bs, uses, key, exp, m.c)
	m.m[key] = tracker
	m.lock.Unlock()
	time.AfterFunc(time.Until(exp), func() {
		m.lock.Lock()
		delete(m.m, key)
		m.lock.Unlock()
	})
	return key, nil
}

func (m *redisUsageTrackerManager) Get(key string) (UsageTracker, bool) {
	m.lock.RLock()
	t, ok := m.m[key]
	m.lock.RUnlock()
	if !ok {
		ttlMs, err := m.c.PTTL(context.Background(), key).Result()
		if err == redis.Nil {
			return nil, false
		} else if err != nil {
			// TODO: Log error
			return nil, false
		} else {
			exp := time.Duration(ttlMs) * time.Millisecond
			return NewRedisUsageTracker(m.bs, 0, key, time.Now().Add(exp), m.c), true
		}
	}
	return t, true
}