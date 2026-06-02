package handlers

import (
	"fmt"
	"go-serve/utils"
	"net/http"
	"os"
	"time"
)

var testGarbage = make([]byte, 128*1024)

func TestHandler(
	w http.ResponseWriter,
	req *http.Request,
	logThreshold int,
	logChan chan<- utils.LogMessage,
) {
	size := 50
	start := time.Now()
	status := http.StatusOK
	bytes := 0
	var outputErr error

	defer func() {
		utils.LogRequest(
			utils.LogMessage{
				StartTime: start,
				Duration:  time.Since(start),
				URL:       req.URL.Path,
				Method:    req.Method,
				Status:    status,
				Size:      bytes,
				Error:     outputErr,
			},
			logChan,
			logThreshold,
			req.Header.Get("Logging"),
		)
	}()

	if !utils.Auth(&status, &outputErr, req.Header.Get("Authorization"), "GET /test route", w) {
		return
	}

	if mb := req.URL.Query().Get("mb"); mb != "" {
		n, err := fmt.Sscanf(mb, "%d", &size)

		if err != nil || n == 0 {
			fmt.Fprintln(os.Stderr, "Error while scanning query string for size")
		}
	}

	rc := http.NewResponseController(w)
	err := rc.SetWriteDeadline(time.Time{})

	if err != nil {
		status = http.StatusHTTPVersionNotSupported
		outputErr = err
		http.Error(w, outputErr.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size*1024*1024))

	for i := 0; i < size*8; i++ {
		n, err := w.Write(testGarbage)

		if err != nil {
			status = http.StatusBadGateway
			outputErr = err
			http.Error(w, outputErr.Error(), status)
			break
		}

		bytes += n
	}
}
