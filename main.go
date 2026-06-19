package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"net/http"

	"github.com/Tahaa-Dev/go-serve/handlers"
	"github.com/Tahaa-Dev/go-serve/sys"
	"github.com/Tahaa-Dev/go-serve/utils"

	// #nosec G108 -- ppprof server is wrapped in auth middleware and is on local network on internal 8081 port
	_ "net/http/pprof"
	"os"
	"time"
)

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
	switch logLevel {
	case "Error":
		logThreshold = 400
	case "Warn":
		logThreshold = 300
	case "Info":
		logThreshold = 200
	default:
	}

	cache := utils.NewCache(cacheCap)

	logChan := make(chan utils.LogMessage, 32*1024)
	logBuf := bufio.NewWriterSize(os.Stderr, 1024*1024)

	go utils.WriteLogs(logChan, logBuf, 10, 2500)
	defer close(logChan)

	serverMux := http.NewServeMux()

	serverMux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		handlers.RequestHandler(
			w,
			req,
			utils.ReqHandlerOpts{
				Dir:   dir,
				Cache: &cache,
			},
		)
	})

	serverMux.HandleFunc("POST /", func(w http.ResponseWriter, req *http.Request) {
		handlers.PostRequestHandler(
			w,
			req,
			utils.ReqHandlerOpts{
				Dir:   dir,
				Cache: &cache,
			},
		)
	})

	serverMux.HandleFunc("PUT /", func(w http.ResponseWriter, req *http.Request) {
		handlers.PutRequestHandler(
			w,
			req,
			utils.ReqHandlerOpts{
				Dir:   dir,
				Cache: &cache,
			},
		)
	})

	serverMux.HandleFunc("DELETE /", func(w http.ResponseWriter, req *http.Request) {
		handlers.DeleteRequestHandler(
			w,
			req,
			utils.ReqHandlerOpts{
				Dir:   dir,
				Cache: &cache,
			},
		)
	})

	go startPprof(logChan, logThreshold)

	server := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := utils.NewLogState()

			realm := ""
			switch r.Method {
			case "GET":
				state.CheckAuth = false
			case "POST":
				realm = "POST / Route"
			case "PUT":
				realm = "PUT / Route"
			case "DELETE":
				realm = "DELETE / Route"
			default:
			}

			utils.LogMiddleware(serverMux, logChan, logThreshold, &state, realm).
				ServeHTTP(&utils.StateResW{State: &state, W: w}, r)
		}),
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
