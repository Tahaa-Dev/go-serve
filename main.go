package main

import (
	"bufio"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	// #nosec G108 -- ppprof server is wrapped in auth middleware and is on local network on internal 8081 port
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var testGarbage = make([]byte, 128*1024)

var authVar string

func init() {
	isSet := false

	if authVar, isSet = os.LookupEnv("GO_SERVE_AUTH"); !isSet {
		fmt.Fprintln(
			os.Stderr,
			"Env Var 'GO_SERVE_AUTH' not found.\n • Set Var to a secure auth token for GET /test route authorization",
		)
		os.Exit(1)
	}
}

func Auth(status *int, err *error, authHeader string, reqName string, w http.ResponseWriter) bool {
	if subtle.ConstantTimeCompare(
		[]byte(authHeader),
		[]byte("Bearer "+authVar),
	) == 0 {
		errStr := "unauthorized " + reqName + " request"
		*status = http.StatusUnauthorized
		*err = errors.New(errStr)

		w.Header().Set("WWW-Authenticate", "Bearer realm=\"Test\"")
		http.Error(w, errStr, *status)

		return false
	}

	return true
}

func TestHandler(
	w http.ResponseWriter,
	req *http.Request,
	logThreshold int,
	logChan chan<- LogMessage,
) {
	size := 50
	start := time.Now()
	status := http.StatusOK
	bytes := 0
	var outputErr error

	defer func() {
		logRequest(
			LogMessage{
				start,
				time.Since(start),
				req.URL.Path,
				req.Method,
				status,
				bytes,
				outputErr,
			},
			logChan,
			logThreshold,
			req.Header.Get("Logging"),
		)
	}()

	if !Auth(&status, &outputErr, req.Header.Get("Authorization"), "GET /test route", w) {
		return
	}

	if mb := req.URL.Query().Get("mb"); mb != "" {
		n, err := fmt.Sscanf(mb, "%d", &size)

		if err != nil || n == 0 {
			fmt.Fprintln(os.Stderr, "Error while scanning query string for size")
		}
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		status = http.StatusHTTPVersionNotSupported
		outputErr = http.ErrNotSupported
		http.Error(w, "HTTP/2 connections not supported for testing. Use HTTP/1.x", status)
		return
	}

	conn, rw, err := hijacker.Hijack()

	if err != nil {
		status = http.StatusHTTPVersionNotSupported
		outputErr = err
		http.Error(w, outputErr.Error(), status)
		return
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to close connection to hijacked test request")
		}
	}()

	_ = conn.SetWriteDeadline(time.Time{})

	_, err = fmt.Fprintf(
		rw,
		"HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n",
		size*1024*1024,
	)
	if err != nil {
		status = http.StatusBadGateway
		outputErr = err
		return
	}

	err = rw.Flush()
	if err != nil {
		status = http.StatusBadGateway
		outputErr = err
		return
	}

	for i := 0; i < size*8; i++ {
		n, err := rw.Write(testGarbage)

		if err != nil {
			status = http.StatusBadGateway
			outputErr = err
			break
		}

		bytes += n
	}
}

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
	Error     error
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
	size := 0
	var cachedEntry *CacheEntry
	var outputErr error

	defer func() {
		logRequest(
			LogMessage{
				start,
				time.Since(start),
				req.URL.Path,
				req.Method,
				status,
				size,
				outputErr,
			},
			opts.LogChan,
			opts.LogThreshold,
			req.Header.Get("Logging"),
		)
	}()

	safePath := filepath.Clean(req.URL.Path)
	file := filepath.Join(opts.Dir, safePath)

	if opts.CacheEnabled {
		cachedFile := opts.Cache.Get(&file)

		checkCache := func() {
			bytes, err := w.Write(cachedFile.data)
			size = bytes

			if err != nil {
				status = http.StatusBadGateway
				outputErr = err
				http.Error(w, outputErr.Error(), status)
			}
		}

		if cachedFile.IsLoaded {
			cachedFile.Mu.RLock()
			checkCache()
			cachedFile.Mu.RUnlock()
			return
		}

		cachedFile.Mu.Lock()
		cachedEntry = cachedFile
		defer cachedEntry.Mu.Unlock()

		// double check if another goroutine built the cache while
		// this goroutine was waiting for the write lock
		if cachedEntry.IsLoaded {
			checkCache()
			return
		}
	}

	// #nosec G304 -- path is sanitized before cache check
	openFile, err := os.OpenFile(file, os.O_RDONLY, 0400)
	if err != nil {
		status = http.StatusNotFound
		outputErr = err
		http.Error(w, outputErr.Error(), status)
		return
	}

	defer func() {
		err := openFile.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to close file:", file)
		}
	}()

	if fileInfo, err := openFile.Stat(); err == nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	} else {
		fmt.Fprintf(
			os.Stderr,
			"Error while setting Content-Length header for request to path: %s\n • Error Message: %s",
			file,
			err,
		)
	}

	buf := make([]byte, 128*1024)
	for {
		bytes, err := openFile.Read(buf)

		if bytes == 0 {
			break
		}

		if err != nil && err != io.EOF {
			status = http.StatusInternalServerError
			outputErr = err
			http.Error(w, outputErr.Error(), status)
			return
		}

		bytesWritten, err := w.Write(buf[:bytes])

		if err != nil {
			status = http.StatusBadGateway
			outputErr = err
			http.Error(w, outputErr.Error(), status)
			return
		}

		if opts.CacheEnabled {
			cachedEntry.data = append(cachedEntry.data, buf[:bytes]...)
		}

		size += bytesWritten
	}

	cachedEntry.IsLoaded = true
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

func PprofMiddleware(
	next http.Handler,
	logChan chan<- LogMessage,
	logThreshold int,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		status := http.StatusOK
		bytes := 0
		var outputErr error

		defer func() {
			logRequest(
				LogMessage{
					start,
					time.Since(start),
					req.URL.Path,
					req.Method,
					status,
					bytes,
					outputErr,
				},
				logChan,
				logThreshold,
				req.Header.Get("Logging"),
			)
		}()

		if !Auth(
			&status,
			&outputErr,
			req.Header.Get("Authorization"),
			"pprof diagnostics route",
			w,
		) {
			return
		}

		next.ServeHTTP(w, req)
	})
}

func main() {
	port := ""
	dir := ""
	cacheEnabled := false
	logLevel := ""
	flag.StringVar(&port, "p", "8000", "Serve on custom port (go-serve -p 3000)\n •")
	flag.StringVar(&dir, "d", ".", "Directory to serve (go-serve -d ./website)\n •")
	flag.BoolVar(&cacheEnabled, "c", false, "Enable caching files in memory (go-serve -c)\n •")
	flag.StringVar(
		&logLevel,
		"l",
		"Warn",
		"Set global log level threshold.\nOverrides Logging header in requests if Logging header has a higher log level threshold (go-serve -l Info)\n • Options: Error/Info/Warn",
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

				errStr := "\n"
				if msg.Error != nil {
					errStr = fmt.Sprintf(" | Error: %s\n", msg.Error)
				}

				_, err := fmt.Fprintf(
					logBuf,
					"[%s] %s %s: Status: %d | Size: %d | Time: %s%s",
					msg.StartTime.Local().Format("15:04:05"),
					msg.Method,
					msg.URL,
					msg.Status,
					msg.Size,
					msg.Duration,
					errStr,
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
			fmt.Fprintln(os.Stderr, "Error while flushing log buffer to Stderr")

			os.Exit(1)
		}
	}()

	serverMux := http.NewServeMux()
	serverMux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		RequestHandler(w, req, ReqHandlerOpts{dir, logThreshold, logChan, &cache, cacheEnabled})
	})
	serverMux.HandleFunc("GET /test", func(w http.ResponseWriter, req *http.Request) {
		TestHandler(w, req, logThreshold, logChan)
	})

	go func() {
		fmt.Fprintln(os.Stderr, "Started diagnostics server on http://localhost:8081/debug/pprof/")

		server := &http.Server{
			Addr:              "localhost:8081",
			Handler:           PprofMiddleware(http.DefaultServeMux, logChan, logThreshold),
			ReadHeaderTimeout: 3 * time.Second,
		}

		err := server.ListenAndServe()
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"Error while starting diagnostics server on port 8081\n • Error Message: %s\n",
				err,
			)

			return
		}
	}()

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           serverMux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second, // a typical request body isn't very large
		WriteTimeout:      15 * time.Second,
	}

	fmt.Fprintf(
		os.Stderr,
		"Serving directory %s on http://localhost:%s\n",
		dir,
		port,
	)

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
