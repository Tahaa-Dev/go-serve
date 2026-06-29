package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/Tahaa-Dev/go-serve/handlers"
	"github.com/Tahaa-Dev/go-serve/sys"
	"github.com/Tahaa-Dev/go-serve/utils"

	// #nosec G108 -- ppprof server is wrapped in auth middleware and is on local network on internal 8081 port
	_ "net/http/pprof"
	"os"
	"time"
)

var logChan = make(chan utils.LogMessage, 32*1024)

func startPprof(logChan chan<- utils.LogMessage, logThreshold int) {
	fmt.Fprintln(os.Stderr, "Started diagnostics server on http://localhost:8081/debug/pprof/")
	state := utils.NewLogState()
	server := &http.Server{
		Addr: "localhost:8081",
		Handler: utils.LogMiddleware(
			http.DefaultServeMux,
			logChan,
			logThreshold,
			&state,
			"Diagnostics",
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
}

func newMux(dir string, cache *utils.Cache, logThreshold int) *http.ServeMux {
	serverMux := http.NewServeMux()

	serverMux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		state := utils.NewLogState()
		state.CheckAuth = false

		utils.LogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlers.RequestHandler(
				&utils.StateResW{State: &state, W: w},
				r,
				utils.ReqHandlerOpts{
					Dir:   dir,
					Cache: cache,
				},
			)
		}), logChan, logThreshold, &state, "GET / Route",
		).ServeHTTP(w, req)
	})

	serverMux.HandleFunc("POST /", func(w http.ResponseWriter, req *http.Request) {
		state := utils.NewLogState()

		utils.LogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlers.PostRequestHandler(
				&utils.StateResW{State: &state, W: w},
				r,
				utils.ReqHandlerOpts{
					Dir:   dir,
					Cache: cache,
				},
			)
		}), logChan, logThreshold, &state, "POST / Route",
		).ServeHTTP(w, req)
	})

	serverMux.HandleFunc("PUT /", func(w http.ResponseWriter, req *http.Request) {
		state := utils.NewLogState()

		utils.LogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlers.PutRequestHandler(
				&utils.StateResW{State: &state, W: w},
				r,
				utils.ReqHandlerOpts{
					Dir:   dir,
					Cache: cache,
				},
			)
		}), logChan, logThreshold, &state, "PUT / Route",
		).ServeHTTP(w, req)
	})

	serverMux.HandleFunc("DELETE /", func(w http.ResponseWriter, req *http.Request) {
		state := utils.NewLogState()

		utils.LogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlers.DeleteRequestHandler(
				&utils.StateResW{State: &state, W: w},
				r,
				utils.ReqHandlerOpts{
					Dir:   dir,
					Cache: cache,
				},
			)
		}), logChan, logThreshold, &state, "DELETE / Route",
		).ServeHTTP(w, req)
	})

	return serverMux
}

func main() {
	port := ""
	dir := ""
	cacheCap := uint(0)
	maxConcurrentReq := uint64(0)
	logLevel := ""
	flag.StringVar(&port, "p", "8000", "Serve on custom port (go-serve -p 3000)\n•")
	flag.StringVar(&dir, "d", ".", "Directory to serve (go-serve -d ./website)\n•")
	flag.UintVar(&cacheCap, "c", 64, "Specify the limit of cache entries (go-serve -c 128)\n•")
	flag.Uint64Var(
		&maxConcurrentReq,
		"m",
		0,
		"Sets system rlimit on Unix, 0 means system limit (go-serve -m 1024)\n•",
	)
	flag.StringVar(
		&logLevel,
		"l",
		"Warn",
		"Set global log level threshold.\nOverrides Logging header in requests if Logging header has a higher log level threshold (go-serve -l Info)\n• Options: Error/Info/Warn",
	)
	flag.Parse()

	if err := sys.SetRLimit(maxConcurrentReq); err != nil {
		fmt.Fprintln(
			os.Stderr,
			"Warning: Failed to set system rlimit\n Error Message:",
			err.Error(),
		)
	}

	logThreshold := 300
	switch strings.ToLower(logLevel) {
	case "error":
		logThreshold = 400
	case "warn":
		logThreshold = 300
	case "info":
		logThreshold = 200
	default:
	}

	cache := utils.NewCache(cacheCap)

	logBuf := bufio.NewWriterSize(os.Stderr, 1024*1024)

	go utils.WriteLogs(logChan, logBuf, 10, 2500)
	defer close(logChan)

	go startPprof(logChan, logThreshold)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newMux(dir, &cache, logThreshold),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second, // a typical request body isn't very large
		WriteTimeout:      15 * time.Second,
		ConnState: func(c net.Conn, cs http.ConnState) {
			if cs == http.StateIdle {
				if conn, ok := c.(*net.TCPConn); ok {
					_ = conn.SetLinger(0)
				}
			}
		},
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
