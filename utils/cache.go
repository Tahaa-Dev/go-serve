package utils

import (
	"sync"
)

type tmp struct {
}

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
	LFUBuckets [64]map[string]tmp
	Cap        uint
	Size       uint
}

func NewCache(cacheCap uint) Cache {
	buckets := [64]map[string]tmp{}

	alloc := uint(0)
	if cacheCap <= 64 {
		alloc = cacheCap * 8
	} else {
		alloc = cacheCap * 4
	}

	for i := range 64 {
		// pre-allocate 8 times the cache capacity per LFUBucket
		buckets[i] = make(map[string]tmp, alloc)
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

	delete(c.LFUBuckets[oldIdx], *file)
	c.LFUBuckets[newIdx][*file] = tmp{}

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
	if entry.Data == nil {
		c.Mu.Lock()

		if c.Size > c.Cap {
			c.evict()
		}

		c.LFUBuckets[c.MinFreq][*file] = tmp{}
		c.Size++
		c.Mu.Unlock()
	}

	arr := make([]byte, len(entry.Data)+len(data))
	copy(arr, entry.Data)
	copy(arr[len(entry.Data):], data)
	entry.Data = arr
}

func (c *Cache) Delete(file *string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	entry, exists := c.Files[*file]
	if exists {
		delete(c.LFUBuckets[c.MinFreq], *file)

		if entry.Freq == c.MinFreq && len(c.LFUBuckets[c.MinFreq]) == 0 {
			c.findNextBucket()
		}

		entry.Mu.Lock()
		delete(c.Files, *file)
		entry.Mu.Unlock()
		c.Size--
	}
}

func (c *Cache) evict() {
	for filename := range c.LFUBuckets[c.MinFreq] {
		delete(c.LFUBuckets[c.MinFreq], filename)
		delete(c.Files, filename)
		break
	}

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
	// cache is completely empty
	c.MinFreq = 0
}
