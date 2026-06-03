package main

import (
	"bufio"
	"flag"
	"fmt"
	"go-serve/handlers"
	"go-serve/utils"
	"net/http"
	// #nosec G108 -- ppprof server is wrapped in auth middleware and is on local network on internal 8081 port
	_ "net/http/pprof"
	"os"
	"sync"
	"time"
)

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

	var cache utils.Cache
	if cacheEnabled {
		cache = utils.Cache{Mu: sync.Mutex{}, Files: make(map[string]*utils.CacheEntry)}
	}

	logChan := make(chan utils.LogMessage, 16*1024)
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
					msg.StartTime.Format("15:04:05"),
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
		state := utils.LogState{
			StartTime: time.Now(),
			Status:    http.StatusOK,
			Size:      0,
			Error:     nil,
			CheckAuth: false,
		}

		utils.LogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlers.RequestHandler(
				w,
				r,
				utils.ReqHandlerOpts{
					Dir:          dir,
					Cache:        &cache,
					CacheEnabled: cacheEnabled,
				},
				&state,
			)
		}), logChan, logThreshold, &state).ServeHTTP(w, req)
	})
	serverMux.HandleFunc("GET /test", func(w http.ResponseWriter, req *http.Request) {
		state := utils.LogState{
			StartTime: time.Now(),
			Status:    http.StatusOK,
			Size:      0,
			Error:     nil,
			CheckAuth: true,
		}

		utils.LogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlers.TestHandler(
				w,
				r,
				&state,
			)
		}), logChan, logThreshold, &state).ServeHTTP(w, req)
	})

	go func() {
		fmt.Fprintln(os.Stderr, "Started diagnostics server on http://localhost:8081/debug/pprof/")

		server := &http.Server{
			Addr: "localhost:8081",
			Handler: utils.LogMiddleware(
				http.DefaultServeMux,
				logChan,
				logThreshold,
				&utils.LogState{
					StartTime: time.Now(),
					Status:    http.StatusOK,
					Size:      0,
					Error:     nil,
					CheckAuth: true,
				},
			),
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
