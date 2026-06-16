package utils

import (
	"sync"
	"sync/atomic"
)

type tmp struct {
}

type CacheEntry struct {
	Mu          *sync.RWMutex
	IsLoaded    bool
	Data        []byte
	Freq        atomic.Uint32
	ContentType string
}

type LFUBucket struct {
	Bucket map[string]tmp
	lock   *sync.RWMutex
}

type Cache struct {
	Mu         *sync.Mutex
	Files      map[string]*CacheEntry
	MinFreq    atomic.Uint32
	LFUBuckets [64]LFUBucket
	Cap        uint
	Size       atomic.Uint64
}

func NewCache(cacheCap uint) Cache {
	buckets := [64]LFUBucket{}

	for i := range 64 {
		// pre-allocate 8 times the cache capacity per LFUBucket
		buckets[i].Bucket = make(map[string]tmp, cacheCap*8)
		buckets[i].lock = &sync.RWMutex{}
	}

	return Cache{
		Mu:         &sync.Mutex{},
		Files:      make(map[string]*CacheEntry, cacheCap),
		MinFreq:    atomic.Uint32{},
		LFUBuckets: buckets,
		Cap:        cacheCap,
		Size:       atomic.Uint64{},
	}
}

func (c *Cache) Get(file *string) *CacheEntry {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	entry, exists := c.Files[*file]
	if !exists {
		entry = &CacheEntry{
			&sync.RWMutex{},
			false,
			nil,
			atomic.Uint32{},
			"NOT ADDED",
		}
		entry.Freq.Store(c.MinFreq.Load())
		c.Files[*file] = entry

		return c.Files[*file]
	}

	oldIdx := entry.Freq.Load()
	newIdx := (oldIdx + 1) % 64

	c.LFUBuckets[oldIdx].lock.Lock()
	delete(c.LFUBuckets[oldIdx].Bucket, *file)
	c.LFUBuckets[newIdx].Bucket[*file] = tmp{}
	c.LFUBuckets[oldIdx].lock.Unlock()

	minFreq := c.MinFreq.Load()
	c.LFUBuckets[minFreq].lock.RLock()
	if oldIdx == minFreq && len(c.LFUBuckets[minFreq].Bucket) == 0 {
		c.MinFreq.Store(newIdx)
	}
	c.LFUBuckets[minFreq].lock.RUnlock()

	entry.Freq.Store(newIdx)

	return entry
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
		if uint(c.Size.Load()) > c.Cap {
			c.evict()
		}

		minFreq := c.MinFreq.Load()
		c.LFUBuckets[minFreq].lock.Lock()
		c.LFUBuckets[minFreq].Bucket[*file] = tmp{}
		c.LFUBuckets[minFreq].lock.Unlock()
		c.Size.Add(1)
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
		minFreq := c.MinFreq.Load()
		c.LFUBuckets[minFreq].lock.Lock()
		delete(c.LFUBuckets[minFreq].Bucket, *file)
		c.LFUBuckets[minFreq].lock.Unlock()

		c.LFUBuckets[minFreq].lock.RLock()
		if entry.Freq.Load() == minFreq && len(c.LFUBuckets[minFreq].Bucket) == 0 {
			c.findNextBucket()
		}
		c.LFUBuckets[minFreq].lock.RUnlock()

		entry.Mu.Lock()
		delete(c.Files, *file)
		entry.Mu.Unlock()
		c.Size.Add(^uint64(0))
	}
}

func (c *Cache) evict() {
	minFreq := c.MinFreq.Load()

	c.LFUBuckets[minFreq].lock.Lock()
	func() {
		defer c.LFUBuckets[minFreq].lock.Unlock()

		for filename := range c.LFUBuckets[minFreq].Bucket {
			delete(c.LFUBuckets[minFreq].Bucket, filename)

			c.Mu.Lock()

			entry := c.Files[filename]
			entry.Mu.Lock()
			delete(c.Files, filename)
			entry.Mu.Unlock()

			c.Mu.Unlock()
			break
		}
	}()

	c.Size.Add(^uint64(0))

	// if MinFreq is empty, find the next active bucket and set MinFreq to its idx
	c.LFUBuckets[minFreq].lock.RLock()
	if len(c.LFUBuckets[minFreq].Bucket) == 0 {
		c.findNextBucket()
	}
}

func (c *Cache) findNextBucket() {
	newMin := (c.MinFreq.Load() + 1) % 64

	for i, bucket := range c.LFUBuckets[newMin:] {
		bucket.lock.RLock()

		if len(bucket.Bucket) > 0 {
			bucket.lock.RUnlock()

			c.MinFreq.Store(uint32(i))
			return
		}

		bucket.lock.RUnlock()
	}

	for i, bucket := range c.LFUBuckets[:newMin] {
		bucket.lock.RLock()

		if len(bucket.Bucket) > 0 {
			bucket.lock.RUnlock()

			c.MinFreq.Store(uint32(i))
			return
		}

		bucket.lock.RUnlock()
	}
	// cache is completely empty
	c.MinFreq.Store(uint32(0))
}
