package utils

import (
	"sync"
	"time"
)

type CacheEntry struct {
	Mu          sync.RWMutex
	IsLoaded    bool
	Data        []byte
	ContentType string
}

type Cache struct {
	Mu    sync.Mutex
	Files map[string]*CacheEntry
}

func (c *Cache) Get(file *string) *CacheEntry {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	entry := c.Files[*file]

	if entry == nil {
		c.Files[*file] = &CacheEntry{sync.RWMutex{}, false, nil, "NOT ADDED"}

		return c.Files[*file]
	}

	return entry
}

type LogMessage struct {
	StartTime time.Time
	Duration  time.Duration
	URL       string
	Method    string
	Status    int
	Size      int
	Error     error
}

type ReqHandlerOpts struct {
	Dir          string
	Cache        *Cache
	CacheEnabled bool
}

type LogState struct {
	StartTime time.Time
	Status    int
	Size      int
	Error     error
	CheckAuth bool
}
