package subpub

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestBasicPublishSubscribe тестируем подписку и публикацию
func TestBasicPublishSubscribe(t *testing.T) {
	sp := NewSubPub()

	var wg sync.WaitGroup
	wg.Add(1)

	sub, err := sp.Subscribe("test", func(msg interface{}) {
		defer wg.Done()
		if str, ok := msg.(string); !ok || str != "hello" {
			t.Errorf("unexpected message: %v", msg)
		}
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	err = sp.Publish("test", "hello")
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	waitWithTimeout(&wg, 2*time.Second, t)
	sub.Unsubscribe()
}

// TestMultipleSubscribers тестируем ситуацию, когда подписчиков > 1
func TestMultipleSubscribers(t *testing.T) {
	sp := NewSubPub()

	var wg sync.WaitGroup
	wg.Add(2)

	handler := func(expected string) MessageHandler {
		return func(msg interface{}) {
			defer wg.Done()
			if msg.(string) != expected {
				t.Errorf("expected %v, got %v", expected, msg)
			}
		}
	}

	sub1, _ := sp.Subscribe("topic", handler("data"))
	sub2, _ := sp.Subscribe("topic", handler("data"))

	err := sp.Publish("topic", "data")
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	waitWithTimeout(&wg, 2*time.Second, t)
	sub1.Unsubscribe()
	sub2.Unsubscribe()
}

// TestUnsubscribe тестируем отписку от темы
func TestUnsubscribe(t *testing.T) {
	sp := NewSubPub()

	called := false

	sub, _ := sp.Subscribe("x", func(msg interface{}) {
		called = true
	})

	sub.Unsubscribe()

	err := sp.Publish("x", "should not be delivered")
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if called {
		t.Error("handler was called after unsubscribe")
	}
}

// TestFIFOOrder проверяем, работает ли порядок очереди
func TestFIFOOrder(t *testing.T) {
	sp := NewSubPub()

	var mu sync.Mutex
	var received []int
	var wg sync.WaitGroup
	wg.Add(3)

	sub, _ := sp.Subscribe("fifo", func(msg interface{}) {
		defer wg.Done()
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msg.(int))
	})

	sp.Publish("fifo", 1)
	sp.Publish("fifo", 2)
	sp.Publish("fifo", 3)

	waitWithTimeout(&wg, 2*time.Second, t)

	mu.Lock()
	defer mu.Unlock()
	expected := []int{1, 2, 3}
	for i := range expected {
		if received[i] != expected[i] {
			t.Errorf("expected %v, got %v", expected, received)
		}
	}

	sub.Unsubscribe()
}

// TestSlowSubscriber тестируем, что медленный не тормозит быстрых
func TestSlowSubscriber(t *testing.T) {
	sp := NewSubPub()

	var fastCalled bool
	var wg sync.WaitGroup
	wg.Add(1)

	_, _ = sp.Subscribe("topic", func(msg interface{}) {
		time.Sleep(1 * time.Second)
	})

	_, _ = sp.Subscribe("topic", func(msg interface{}) {
		defer wg.Done()
		fastCalled = true
	})

	start := time.Now()
	err := sp.Publish("topic", "ping")
	if err != nil {
		t.Fatal(err)
	}

	waitWithTimeout(&wg, 1*time.Second, t)

	if !fastCalled {
		t.Error("fast subscriber was not called")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("slow subscriber blocked publish")
	}
}

// TestCloseBlocks тестируем, что Close ждет завершения хендлеров
func TestCloseBlocks(t *testing.T) {
	sp := NewSubPub()

	var wg sync.WaitGroup
	wg.Add(1)

	_, _ = sp.Subscribe("done", func(msg interface{}) {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
	})

	sp.Publish("done", "msg")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := sp.Close(ctx)
	if err != nil {
		t.Fatalf("expected clean close, got: %v", err)
	}

	waitWithTimeout(&wg, 1*time.Second, t)
}

func waitWithTimeout(wg *sync.WaitGroup, timeout time.Duration, t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("timeout waiting for wait group")
	}
}
