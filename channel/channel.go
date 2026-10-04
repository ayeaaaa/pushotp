package channel

import (
	"context"
	"fmt"
	"sync"
)

type Message struct {
	Title   string
	Content string
}

type Target struct {
	Receiver string
	Config   map[string]string
}

type Channel interface {
	Name() string
	Send(ctx context.Context, target Target, msg Message) error
}

type Factory func() Channel

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

func Register(name string, f Factory) {
	if f == nil {
		panic("channel: nil factory")
	}
	mu.Lock()
	defer mu.Unlock()
	registry[name] = f
}

func New(name string) (Channel, error) {
	mu.RLock()
	f, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("channel: unknown channel %q", name)
	}
	return f(), nil
}

func Registered(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := registry[name]
	return ok
}
