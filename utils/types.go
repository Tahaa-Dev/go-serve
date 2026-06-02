package utils

import (
	"sync"
	"time"
)

type CacheEntry struct {
	Mu       sync.RWMutex
	IsLoaded bool
	Data     []byte
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
		c.Files[*file] = &CacheEntry{sync.RWMutex{}, false, nil}

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
	LogThreshold int
	LogChan      chan<- LogMessage
	Cache        *Cache
	CacheEnabled bool
}
