package sync

import (
	"sync"
)

// MutexMap is a map of mutexes where key is a string and value is a sync.RWMutex.
type OnceMap struct {
	mu      sync.RWMutex
	onceMap map[string]*sync.Once
}

// Returns a pointer to a new instance of MutexMap.
func NewOnceMap() *OnceMap {
	return &OnceMap{
		onceMap: make(map[string]*sync.Once),
	}
}

// Do calls f only once for a given key.
func (o *OnceMap) Do(key string, f func()) {
	o.mu.RLock()
	v, ok := o.onceMap[key]
	o.mu.RUnlock()

	if ok && v != nil {
		return
	}

	o.mu.Lock()

	v, ok = o.onceMap[key]
	if ok && v != nil {
		return
	}
	once := &sync.Once{}
	o.onceMap[key] = once

	o.mu.Unlock()

	once.Do(f)
}
