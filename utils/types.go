package utils

import (
	"bytes"
	"net/http"
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
	buckets := make([][]byte, 64)

	alloc := uint(0)
	if cacheCap <= 64 {
		alloc = cacheCap * 8
	} else {
		alloc = cacheCap * 4
	}

	for i := range 64 {
		// pre-allocate 8 times the cache capacity per LFUBucket
		buckets[i] = make([]byte, 0, alloc)
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

	entry, exists := c.Files[*file]
	if !exists {
		c.Files[*file] = &CacheEntry{
			sync.RWMutex{},
			false,
			nil,
			c.MinFreq,
			"NOT ADDED",
		}

		return c.Files[*file]
	}

	oldIdx := entry.Freq
	newIdx := (oldIdx + 1) % 64

	c.deleteLocked(file, oldIdx)

	if len(c.LFUBuckets[newIdx]) > 0 {
		c.LFUBuckets[newIdx] = append(c.LFUBuckets[newIdx], 0)
	}
	c.LFUBuckets[newIdx] = append(c.LFUBuckets[newIdx], []byte(*file)...)

	if oldIdx == c.MinFreq && len(c.LFUBuckets[c.MinFreq]) == 0 {
		c.MinFreq = newIdx
	}

	c.Files[*file].Freq = newIdx

	return c.Files[*file]
}

func (c *Cache) Update(file *string, data []byte, entry *CacheEntry, truncate bool) {
	if entry.Data == nil || !truncate {
		c.Add(file, data, entry)
	} else {
		entry.Data = data
	}
}

func (c *Cache) Add(file *string, data []byte, entry *CacheEntry) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if entry.Data == nil {
		if c.Size == c.Cap {
			c.evict()
		}

		if len(c.LFUBuckets[c.MinFreq]) > 0 {
			c.LFUBuckets[c.MinFreq] = append(c.LFUBuckets[c.MinFreq], 0)
		}

		c.LFUBuckets[c.MinFreq] = append(c.LFUBuckets[c.MinFreq], []byte(*file)...)
		c.Size++
	}

	entry.Data = append(entry.Data, data...)
}

func (c *Cache) Delete(file *string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	entry, exists := c.Files[*file]
	if exists {
		c.deleteLocked(file, entry.Freq)

		if entry.Freq == c.MinFreq && len(c.LFUBuckets[c.MinFreq]) == 0 {
			c.findNextBucket()
		}

		entry.Mu.Lock()
		delete(c.Files, *file)
		entry.Mu.Unlock()
		c.Size--
	}
}

func (c *Cache) deleteLocked(file *string, idx int) {
	fileBytes := []byte(*file)
	startIdx := bytes.Index(c.LFUBuckets[idx], fileBytes)
	endIdx := startIdx + len(fileBytes)

	if endIdx+1 > len(c.LFUBuckets[idx]) {
		endIdx--
	}

	c.LFUBuckets[idx] = append(
		c.LFUBuckets[idx][:startIdx],
		c.LFUBuckets[idx][endIdx+1:]...)
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
		c.findNextBucket()
	}
}

func (c *Cache) findNextBucket() {
	newMin := (c.MinFreq + 1) % 64
	for i, bucket := range c.LFUBuckets[newMin:] {
		if len(bucket) > 0 {
			c.MinFreq = i
			return
		}
	}

	for i, bucket := range c.LFUBuckets[:newMin] {
		if len(bucket) > 0 {
			c.MinFreq = i
			return
		}
	}
}

type ReqHandlerOpts struct {
	Dir   string
	Cache *Cache
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

type LogState struct {
	StartTime time.Time
	Status    int
	Size      int
	Error     error
	CheckAuth bool
}

func NewLogState(chechAuth bool) LogState {
	return LogState{
		StartTime: time.Now(),
		Status:    http.StatusOK,
		Size:      0,
		Error:     nil,
		CheckAuth: chechAuth,
	}
}
