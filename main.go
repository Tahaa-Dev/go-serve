package main

import (
	"bufio"
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
	Mu       sync.RWMutex
	IsLoaded bool
	data     []byte
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
}

type ReqHandlerOpts struct {
	Dir          string
	LogThreshold int
	LogChan      chan<- LogMessage
	Cache        *Cache
	CacheEnabled bool
}

func RequestHandler(
	w http.ResponseWriter,
	req *http.Request,
	opts ReqHandlerOpts,
) {
	start := time.Now()
	status := http.StatusOK
	var cachedEntry *CacheEntry

	safePath := filepath.Clean(req.URL.Path)
	file := filepath.Join(opts.Dir, safePath)

	if opts.CacheEnabled {
		cachedFile := opts.Cache.Get(&file)

		cachedFile.Mu.RLock()

		checkCache := func() {
			bytes, err := w.Write(cachedFile.data)

			if err != nil {
				if bytes > 0 {
					status = http.StatusPartialContent
				} else {
					status = http.StatusBadGateway
				}
			}

			logRequest(
				LogMessage{start, time.Since(start), req.URL.Path, req.Method, status, bytes},
				opts.LogChan,
				opts.LogThreshold,
				req.Header.Get("Logging"),
			)
		}

		if cachedFile.IsLoaded {
			checkCache()
			cachedFile.Mu.RUnlock()
			return
		}

		cachedFile.Mu.RUnlock()
		cachedFile.Mu.Lock()
		cachedEntry = cachedFile
		defer cachedEntry.Mu.Unlock()

		// double check if another goroutine built the cache
		// while this goroutine was waiting for the write lock
		if cachedEntry.IsLoaded {
			checkCache()
			return
		}
	}

	// #nosec G304 -- path is sanitized before cache check
	openFile, err := os.OpenFile(file, os.O_RDONLY, 0400)
	if err != nil {
		status = http.StatusNotFound
		logRequest(
			LogMessage{start, time.Since(start), req.URL.Path, req.Method, status, 0},
			opts.LogChan,
			opts.LogThreshold,
			req.Header.Get("Logging"),
		)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	defer func() {
		err := openFile.Close()
		if err != nil {
			status = http.StatusInternalServerError
			logRequest(
				LogMessage{start, time.Since(start), req.URL.Path, req.Method, status, 0},
				opts.LogChan,
				opts.LogThreshold,
				req.Header.Get("Logging"),
			)
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
			status = http.StatusInternalServerError
			logRequest(
				LogMessage{start, time.Since(start), req.URL.Path, req.Method, status, 0},
				opts.LogChan,
				opts.LogThreshold,
				req.Header.Get("Logging"),
			)
			break
		}

		bytesWritten, err := w.Write(buf[:bytes])

		if err != nil {
			if bytesWritten > 0 {
				status = http.StatusPartialContent
				logRequest(
					LogMessage{start, time.Since(start), req.URL.Path, req.Method, status, bytes},
					opts.LogChan,
					opts.LogThreshold,
					req.Header.Get("Logging"),
				)
			} else {
				status = http.StatusBadGateway
				logRequest(
					LogMessage{start, time.Since(start), req.URL.Path, req.Method, status, bytes},
					opts.LogChan,
					opts.LogThreshold,
					req.Header.Get("Logging"),
				)
			}
			return
		}

		if opts.CacheEnabled {
			cachedEntry.data = append(cachedEntry.data, buf[:bytes]...)
		}

		size += bytesWritten
	}

	cachedEntry.IsLoaded = true
	logRequest(
		LogMessage{start, time.Since(start), req.URL.Path, req.Method, status, size},
		opts.LogChan,
		opts.LogThreshold,
		req.Header.Get("Logging"),
	)
}

func logRequest(
	msg LogMessage,
	ch chan<- LogMessage,
	threshold int,
	logHeader string,
) {
	switch logHeader {
	case "Error":
		if threshold > 400 {
			threshold = 400
		}
	case "Warn":
		if threshold > 300 {
			threshold = 300
		}
	case "Info":
		if threshold > 200 {
			threshold = 200
		}
	default:
	}

	if msg.Status >= threshold {
		ch <- msg
	}
}

func main() {
	port := ""
	dir := ""
	logLevel := ""
	cacheEnabled := false
	flag.StringVar(
		&port,
		"p",
		"8000",
		"Serve on custom port (go-serve -p 3000)\n •",
	)
	flag.StringVar(
		&dir,
		"d",
		".",
		"Directory to serve (go-serve -d ./website)\n •",
	)
	flag.StringVar(
		&logLevel,
		"l",
		"Warn",
		"Set global log level threshold.\nOverrides Logging header in requests if Logging header has a higher log level threshold (go-serve -l Info)\n • Options: Error/Info/Warn",
	)
	flag.BoolVar(
		&cacheEnabled,
		"c",
		false,
		"Enable caching files in memory (go-serve -c)\n •",
	)
	flag.Parse()

	logThreshold := 300
	switch logLevel {
	case "Error":
		logThreshold = 400
	case "Warn":
		logThreshold = 300
	case "Info":
		logThreshold = 200
	default:
	}

	var cache Cache
	if cacheEnabled {
		cache = Cache{sync.Mutex{}, make(map[string]*CacheEntry)}
	}

	logChan := make(chan LogMessage, 16*1024)
	logBuf := bufio.NewWriterSize(os.Stderr, 1024*1024)
	writeLogs := func() {
		ticker := time.NewTicker(2500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-logChan:
				if !ok {
					return
				}
				_, err := fmt.Fprintf(
					logBuf,
					"[%s] %s %s: Status: %d | Size: %d | Time: %s\n",
					msg.StartTime.Local().Format("15:04:05"),
					msg.Method,
					msg.URL,
					msg.Status,
					msg.Size,
					msg.Duration,
				)

				if err != nil {
					fmt.Fprintln(
						os.Stderr,
						"Failed to write log at:",
						msg.StartTime.Local().Format("15:04:05"),
					)
				}
			case <-ticker.C:
				err := logBuf.Flush()
				if err != nil {
					fmt.Fprintln(os.Stderr, "Failed to flush logs")
				}
			}
		}
	}

	go writeLogs()

	defer func() {
		close(logChan)
		writeLogs()

		err := logBuf.Flush()
		if err != nil {
			fmt.Fprintln(
				os.Stderr,
				"Error while flushing log buffer to Stderr",
			)

			os.Exit(1)
		}
	}()

	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		RequestHandler(w, req, ReqHandlerOpts{dir, logThreshold, logChan, &cache, cacheEnabled})
	})

	fmt.Fprintf(
		os.Stderr,
		"Serving directory %s on http://localhost:%s\n",
		dir,
		port,
	)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           serverMux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second, // a typical request body isn't very large
		WriteTimeout:      15 * time.Second,
	}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error while starting server on port %s\n • Error Message: %s\n",
			port,
			err,
		)

		os.Exit(1)
	}
}
