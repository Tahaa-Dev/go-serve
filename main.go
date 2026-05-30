package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CacheEntry struct {
	mu       sync.RWMutex
	isLoaded bool
	data     []byte
}

type Cache struct {
	mu    sync.Mutex
	files map[string]*CacheEntry
}

func (c *Cache) Get(file *string) *CacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := c.files[*file]

	if entry == nil {
		c.files[*file] = &CacheEntry{sync.RWMutex{}, false, nil}

		return c.files[*file]
	}

	return entry
}

func RequestHandler(
	w http.ResponseWriter,
	req *http.Request,
	dir *string,
	log int,
	cache *Cache,
	cacheEnabled bool,
) {
	start := time.Now()
	status := http.StatusOK
	var cachedEntry *CacheEntry

	safePath := filepath.Clean(req.URL.Path)
	file := filepath.Join(*dir, safePath)

	if cacheEnabled {
		cachedFile := cache.Get(&file)

		cachedFile.mu.RLock()

		if cachedFile.isLoaded {
			bytes, err := w.Write(cachedFile.data)
			cachedFile.mu.RUnlock()

			if err != nil {
				if bytes > 0 {
					status = http.StatusPartialContent
				} else {
					status = http.StatusBadGateway
				}
			}

			logRequest(req, &start, status, bytes, log)
			return
		}

		cachedFile.mu.RUnlock()
		cachedFile.mu.Lock()
		cachedEntry = cachedFile
		defer cachedEntry.mu.Unlock()

		// double check if another goroutine built the cache
		// while this goroutine was waiting for the write lock
		if cachedEntry.isLoaded {
			bytes, err := w.Write(cachedFile.data)

			if err != nil {
				if bytes > 0 {
					status = http.StatusPartialContent
				} else {
					status = http.StatusBadGateway
				}
			}

			logRequest(req, &start, status, bytes, log)
			return
		}
	}

	openFile, err := os.OpenFile(file, os.O_RDONLY, os.ModePerm)
	if err != nil {
		logRequest(req, &start, http.StatusNotFound, 0, log)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	defer func() {
		err := openFile.Close()
		if err != nil {
			logRequest(req, &start, http.StatusInternalServerError, 0, log)
		}
	}()

	buf := make([]byte, 128*1024)
	size := 0
	for {
		bytes, err := openFile.Read(buf)

		if bytes == 0 {
			break
		}

		if err != nil && err != io.EOF {
			logRequest(req, &start, http.StatusInternalServerError, 0, log)
			break
		}

		bytesWritten, err := w.Write(buf[:bytes])

		if err != nil {
			if bytesWritten > 0 {
				status = http.StatusPartialContent
				logRequest(req, &start, status, bytes, log)
			} else {
				status = http.StatusBadGateway
				logRequest(req, &start, status, bytes, log)
			}
			return
		}

		if cacheEnabled {
			cachedEntry.data = append(cachedEntry.data, buf[:bytes]...)
		}

		size += bytesWritten
	}

	cachedEntry.isLoaded = true
	logRequest(req, &start, status, size, log)
}

func logRequest(req *http.Request, start *time.Time, status int, size int, log int) {
	switch req.Header.Get("Logging") {
	case "Error":
		if log > 400 {
			log = 400
		}
	case "Warn":
		if log > 300 {
			log = 300
		}
	case "Info":
		if log > 200 {
			log = 200
		}
	}

	if status >= log {
		fmt.Fprintf(os.Stderr, "[%s] %s %s: Status: %d | Size: %d | Time: %s\n",
			start.Format("15:04:05"),
			req.Method,
			req.URL,
			status,
			size,
			time.Since(*start),
		)
	}
}

func main() {
	port := ""
	dir := ""
	logLevel := ""
	cacheEnabled := false
	flag.StringVar(&port, "p", "8000", "Serve on custom port (go-serve -p 3000)\n •")
	flag.StringVar(&dir, "d", ".", "Directory to serve (go-serve -d ./website)\n •")
	flag.StringVar(&logLevel, "l", "Warn", "Set global log level threshold.\nOverrides Logging header in requests if Logging header has a higher log level threshold (go-serve -l Info)\n • Options: Error/Info/Warn")
	flag.BoolVar(&cacheEnabled, "c", false, "Enable caching files in memory (go-serve -c)\n •")
	flag.Parse()

	log := 300
	switch logLevel {
	case "Error":
		log = 400
	case "Warn":
		log = 300
	case "Info":
		log = 200
	}

	var cache Cache
	if cacheEnabled {
		cache = Cache{sync.Mutex{}, make(map[string]*CacheEntry)}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		RequestHandler(w, req, &dir, log, &cache, cacheEnabled)
	})

	fmt.Fprintf(os.Stderr, "Serving directory %s on http://localhost:%s\n", dir, port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while starting server listener on port %s\n • Error Message: %s\n", port, err)
	}
}
