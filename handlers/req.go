package handlers

import (
	"fmt"
	"go-serve/utils"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 128*1024)
		return &buf
	},
}

func RequestHandler(
	w http.ResponseWriter,
	req *http.Request,
	opts utils.ReqHandlerOpts,
) {
	start := time.Now()
	status := http.StatusOK
	size := 0
	var cachedEntry *utils.CacheEntry
	var outputErr error

	defer func() {
		utils.LogRequest(
			utils.LogMessage{
				StartTime: start,
				Duration:  time.Since(start),
				URL:       req.URL.Path,
				Method:    req.Method,
				Status:    status,
				Size:      size,
				Error:     outputErr,
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
			bytes, err := w.Write(cachedFile.Data)
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

	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)

	for {
		bytes, err := openFile.Read((*buf))

		if bytes == 0 {
			break
		}

		if err != nil && err != io.EOF {
			status = http.StatusInternalServerError
			outputErr = err
			http.Error(w, outputErr.Error(), status)
			return
		}

		bytesWritten, err := w.Write((*buf)[:bytes])

		if err != nil {
			status = http.StatusBadGateway
			outputErr = err
			http.Error(w, outputErr.Error(), status)
			return
		}

		if opts.CacheEnabled {
			cachedEntry.Data = append(cachedEntry.Data, (*buf)[:bytes]...)
		}

		size += bytesWritten
	}

	cachedEntry.IsLoaded = true
}
