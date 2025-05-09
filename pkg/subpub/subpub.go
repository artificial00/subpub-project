package subpub

import (
	"context"
	"errors"
	"sync"
)

type MessageHandler func(msg interface{})

type Subscription interface {
	Unsubscribe()
}

type SubPub interface {
	Subscribe(subject string, cb MessageHandler) (Subscription, error)
	Publish(subject string, msg interface{}) error
	Close(ctx context.Context) error
}

func NewSubPub() SubPub {
	return &subpubImpl{
		subscribers: make(map[string]map[*subscriber]struct{}),
	}
}

type subpubImpl struct {
	mu          sync.RWMutex
	subscribers map[string]map[*subscriber]struct{}
	closed      bool
}

type subscriber struct {
	handler MessageHandler
	ch      chan interface{}
	once    sync.Once
	stop    chan struct{}
}

type subscriptionImpl struct {
	sub    *subscriber
	topic  string
	parent *subpubImpl
}

// Subscribe подписывает подписчика на тему
func (s *subpubImpl) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	if cb == nil {
		return nil, errors.New("callback cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errors.New("subpub is closed")
	}

	sub := &subscriber{
		handler: cb,
		ch:      make(chan interface{}, 128),
		stop:    make(chan struct{}),
	}

	if _, ok := s.subscribers[subject]; !ok {
		s.subscribers[subject] = make(map[*subscriber]struct{})
	}
	s.subscribers[subject][sub] = struct{}{}

	go sub.run()

	return &subscriptionImpl{
		sub:    sub,
		topic:  subject,
		parent: s,
	}, nil
}

// Publish шлет сообщение всем подписчикам темы
func (s *subpubImpl) Publish(subject string, msg interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return errors.New("subpub is closed")
	}

	subs, ok := s.subscribers[subject]
	if !ok {
		return nil
	}

	for sub := range subs {
		select {
		case sub.ch <- msg:
		default:
		}
	}

	return nil
}

// Close завершает работу всех подписчиков с учетом контекста
func (s *subpubImpl) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true

	var wg sync.WaitGroup
	for _, subs := range s.subscribers {
		for sub := range subs {
			wg.Add(1)
			go func(sub *subscriber) {
				defer wg.Done()
				sub.stopOnce()
			}(sub)
		}
	}
	s.subscribers = nil
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// run запускает обработку входящих сообщений подписчиком
func (s *subscriber) run() {
	for {
		select {
		case msg := <-s.ch:
			s.handler(msg)
		case <-s.stop:
			return
		}
	}
}

// stopOnce закрывает подписчика однократно
func (s *subscriber) stopOnce() {
	s.once.Do(func() {
		close(s.stop)
	})
}

// Unsubscribe удаляет подписчика из шины
func (s *subscriptionImpl) Unsubscribe() {
	s.parent.mu.Lock()
	defer s.parent.mu.Unlock()

	if subs, ok := s.parent.subscribers[s.topic]; ok {
		delete(subs, s.sub)
		if len(subs) == 0 {
			delete(s.parent.subscribers, s.topic)
		}
	}

	s.sub.stopOnce()
}
