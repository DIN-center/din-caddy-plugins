package usage

import (
	"context"
	// "errors"
	"testing"
	"github.com/go-redis/redismock/v8"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"runtime"
	// "sync/atomic"
	"sync"
	"time"
)

type MockPubSubClient struct {
	subscribers map[string][]chan string
	mutex       sync.Mutex
}

func NewMockPubSubClient() *MockPubSubClient {
	return &MockPubSubClient{
		subscribers: make(map[string][]chan string),
	}
}

func (mps *MockPubSubClient) Subscribe(ctx context.Context, channel string) <-chan string {
	mps.mutex.Lock()
	defer mps.mutex.Unlock()

	ch := make(chan string, 1)
	mps.subscribers[channel] = append(mps.subscribers[channel], ch)
	return ch
}

func (mps *MockPubSubClient) Publish(ctx context.Context, channel, message string) {
	runtime.Gosched() // Give startSubscriber time to subscribe before we publish
	mps.mutex.Lock()
	defer mps.mutex.Unlock()

	counter := 0

	for _, ch := range mps.subscribers[channel] {
		counter++
		ch <- message
	}
	runtime.Gosched() // Allow subscribing goroutines time to process before we return
}

func TestRedisUsageTracker_Use(t *testing.T) {
	db, mock := redismock.NewClientMock()

	mock.ExpectDecrBy("api_key_usage", int64(10)).SetVal(90)
	mock.ExpectDecrBy("api_key_usage", int64(10)).SetVal(-5)
	mock.ExpectPublish("api_key_usage_notifications", "api_key_usage").SetVal(1)

	tracker := &redisUsageTracker{
		BatchSize: 10,
		Uses:      new(int64),
		Key:       "api_key_usage",
		Client:    db,
		PubSub:    &RedisPubSubClient{db},
	}
	// tracker.Error.Store(nil)

	// First batch decrement
	err := tracker.Use()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), *tracker.Uses)

	// Exceed usage
	for i := 0; i < 18; i++ {
		assert.NoError(t, tracker.Use())
	}
	err = tracker.Use()
	assert.Error(t, err)
	assert.Equal(t, ErrRequestLimit, err)

	mock.ExpectationsWereMet()
}

func TestNotificationManager(t *testing.T) {
	db, _ := redismock.NewClientMock()
	mockPubSub := NewMockPubSubClient()

	manager := NewNotificationManager(mockPubSub)
	tracker := &redisUsageTracker{
		BatchSize: 10,
		Uses:      new(int64),
		Key:       "api_key_usage",
		Client:    db,
		PubSub:    mockPubSub,
	}
	manager.RegisterTracker("api_key_usage", tracker)

	// Simulate Pub/Sub message
	go manager.startSubscriber("api_key_usage_notifications")
	mockPubSub.Publish(context.Background(), "api_key_usage_notifications", "api_key_usage")

	// Verify tracker state
	err := tracker.Use()
	assert.Error(t, err)
	assert.Equal(t, ErrRequestLimit, err)
}

func TestRedisTrackerManager(t *testing.T) {
	db, mock := redismock.NewClientMock()
	mock.ExpectIncr("din_caddy_counter_key").SetVal(1)
	tm := NewRedisTrackerManager(db, 5)
	exp := time.Now().Add(20 * time.Millisecond)
	key, err := tm.Create(5, exp)
	mock.ExpectDecrBy(key, int64(5)).SetVal(0)
	mock.ExpectPTTL(key).SetErr(redis.Nil)
	assert.NoError(t, err)
	tracker, ok := tm.Get(key)
	assert.True(t, ok)
	assert.NoError(t, tracker.Use()) // 4
	assert.NoError(t, tracker.Use()) // 3
	assert.NoError(t, tracker.Use()) // 2
	assert.NoError(t, tracker.Use()) // 1
	assert.NoError(t, tracker.Use()) // 0
	assert.Error(t, tracker.Use())   // -1
	time.Sleep(time.Until(exp))
	_, ok = tm.Get(key)
	assert.False(t, ok)
}