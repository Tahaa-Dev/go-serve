package utils

import (
	"bytes"
	"sync"
	"time"
)

type CacheEntry struct {
	Mu          sync.RWMutex
	IsLoaded    bool
	Data        []byte
	Freq        int
	ContentType string
}

type Cache struct {
	Mu         sync.Mutex
	Files      map[string]*CacheEntry
	MinFreq    int
	LFUBuckets [][]byte
	Cap        uint
	Size       uint
}

func NewCache(cacheCap uint) Cache {
	buckets := make([][]byte, 100)

	for i := range 100 {
		buckets[i] = make([]byte, 0, 128)
	}

	return Cache{
		Mu:         sync.Mutex{},
		Files:      make(map[string]*CacheEntry, cacheCap),
		MinFreq:    0,
		LFUBuckets: buckets,
		Cap:        cacheCap,
		Size:       0,
	}
}

func (c *Cache) Get(file *string) *CacheEntry {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	fileBytes := []byte(*file)

	entry, exists := c.Files[*file]
	if !exists {
		newMin := (c.MinFreq + 1) % 100

		c.Files[*file] = &CacheEntry{
			sync.RWMutex{},
			false,
			nil,
			newMin,
			"NOT ADDED",
		}

		return c.Files[*file]
	}

	oldIdx := entry.Freq
	newIdx := (oldIdx + 1) % 100

	startIdx := bytes.Index(c.LFUBuckets[oldIdx], fileBytes)
	endIdx := startIdx + len(fileBytes)

	if endIdx > len(c.LFUBuckets[oldIdx]) {
		endIdx--
	}

	c.LFUBuckets[oldIdx] = append(
		c.LFUBuckets[oldIdx][:startIdx],
		c.LFUBuckets[oldIdx][endIdx:]...)

	if len(c.LFUBuckets[newIdx]) > 0 {
		c.LFUBuckets[newIdx] = append(c.LFUBuckets[newIdx], 0)
	}
	c.LFUBuckets[newIdx] = append(c.LFUBuckets[newIdx], fileBytes...)

	if oldIdx == c.MinFreq && len(c.LFUBuckets[c.MinFreq]) == 0 {
		c.MinFreq = newIdx
	}

	entry.Freq = newIdx

	return entry
}

func (c *Cache) Add(file *string, data []byte, entry *CacheEntry) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.Size == c.Cap {
		c.evict()
	}

	if entry.Data == nil {
		newMin := (c.MinFreq + 1) % 100

		if len(c.LFUBuckets[newMin]) > 0 {
			c.LFUBuckets[newMin] = append(c.LFUBuckets[newMin], 0)
		}

		c.LFUBuckets[newMin] = append(c.LFUBuckets[newMin], []byte(*file)...)
		c.Size++
	}

	entry.Data = append(entry.Data, data...)
}

func (c *Cache) evict() {
	endIdx := bytes.IndexByte(c.LFUBuckets[c.MinFreq], 0)
	startIdx := endIdx

	if endIdx == -1 {
		endIdx = len(c.LFUBuckets[c.MinFreq]) - 1
		startIdx = endIdx + 1
	}

	delete(c.Files, string(c.LFUBuckets[c.MinFreq][:startIdx]))
	c.LFUBuckets[c.MinFreq] = c.LFUBuckets[c.MinFreq][endIdx+1:]

	c.Size--

	// if MinFreq is empty, find the next active bucket and set MinFreq to its idx
	if len(c.LFUBuckets[c.MinFreq]) == 0 {
		for i, bucket := range c.LFUBuckets[(c.MinFreq+1)%100:] {
			if len(bucket) > 0 {
				c.MinFreq = i
				break
			}
		}
	}
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
	Dir   string
	Cache *Cache
}

type LogState struct {
	StartTime time.Time
	Status    int
	Size      int
	Error     error
	CheckAuth bool
}
