package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func RequestHandler(w http.ResponseWriter, req *http.Request, dir *string, log int) {
	start := time.Now()

	safePath := filepath.Clean(req.URL.Path)
	file := filepath.Join(*dir, safePath)

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

	buf := make([]byte, 256*1024)
	size := 0

	status := http.StatusOK
	for {
		bytes, err := openFile.Read(buf)

		if bytes == 0 {
			break
		}

		if err != nil && err != io.EOF {
			logRequest(req, &start, http.StatusInternalServerError, 0, log)
			break
		}

		bytes, err = w.Write(buf[:bytes])

		if err != nil {
			if bytes > 0 {
				status = http.StatusPartialContent
				logRequest(req, &start, status, bytes, log)
			} else {
				status = http.StatusBadGateway
				logRequest(req, &start, status, bytes, log)
			}
			return
		}

		size += bytes
	}

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
	flag.StringVar(&port, "p", "8000", "Serve on custom port (go-serve -p 3000)\n •")
	flag.StringVar(&dir, "d", ".", "Directory to serve (go-serve -d ./website)\n •")
	flag.StringVar(&logLevel, "l", "Warn", "Set global log level threshold.\nOverrides Logging header in requests if Logging header has a higher log level threshold (go-serve -l Info)\n • Options: Error/Info/Warn")
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

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		RequestHandler(w, req, &dir, log)
	})

	fmt.Fprintf(os.Stderr, "Serving directory %s on http://localhost:%s\n", dir, port)
	http.ListenAndServe(":"+port, nil)
}
