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

func RequestHandler(w http.ResponseWriter, req *http.Request, dir *string) {
	start := time.Now()

	safePath := filepath.Clean(req.URL.Path)
	file := filepath.Join(*dir, safePath)

	openFile, err := os.OpenFile(file, os.O_RDONLY, os.ModePerm)
	if err != nil {
		logRequest(req, &start, http.StatusNotFound, 0)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	defer func() {
		err := openFile.Close()
		if err != nil {
			logRequest(req, &start, http.StatusInternalServerError, 0)
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
			logRequest(req, &start, http.StatusInternalServerError, 0)
			break
		}

		bytes, err = w.Write(buf[:bytes])

		if err != nil {
			if bytes > 0 {
				status = http.StatusPartialContent
				logRequest(req, &start, status, bytes)
			} else {
				status = http.StatusBadGateway
				logRequest(req, &start, status, bytes)
			}
			return
		}

		size += bytes
	}

	logRequest(req, &start, status, size)
}

func logRequest(req *http.Request, start *time.Time, status int, size int) {
	threshold := 300
	switch req.Header.Get("Logging") {
	case "Error":
		threshold = 400
	case "Warn":
		threshold = 300
	case "Info":
		threshold = 200
	}

	if status >= threshold {
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
	flag.StringVar(&port, "p", "8000", "Serve on custom port (go-serve -p 3000)")
	flag.StringVar(&dir, "d", ".", "Directory to serve (go-serve -d ./website)")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		RequestHandler(w, req, &dir)
	})

	fmt.Fprintf(os.Stderr, "Serving directory %s on http://localhost:%s\n", dir, port)
	http.ListenAndServe(":"+port, nil)
}
