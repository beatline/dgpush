package push

import (
	"push-worker/models"
	"sync"
)

var (
	handlers = make(map[int]models.Pusher)
	mu       sync.RWMutex
)

func Register(brand int, p models.Pusher) {
	mu.Lock()
	handlers[brand] = p
	mu.Unlock()
}

func GetHandler(brand int) models.Pusher {
	mu.RLock()
	defer mu.RUnlock()
	return handlers[brand]
}
