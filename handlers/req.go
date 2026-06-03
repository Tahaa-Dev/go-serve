package handlers

import (
	"fmt"
	"go-serve/utils"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
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
	state *utils.LogState,
) {
	var cachedEntry *utils.CacheEntry

	safePath := filepath.Clean(req.URL.Path)
	file := filepath.Join(opts.Dir, safePath)

	w.Header().Set("X-Content-Type-Options", "nosniff")

	if opts.CacheEnabled {
		cachedFile := opts.Cache.Get(&file)

		checkCache := func() {
			contentType := ""

			if cachedFile.ContentType == "NOT ADDED" {
				contentType = http.DetectContentType(cachedFile.Data)
			} else {
				contentType = cachedFile.ContentType
			}

			w.Header().Set("Content-Type", contentType)
			// #nosec G705 -- intentional file server design
			bytes, err := w.Write(cachedFile.Data)
			state.Size = bytes

			if err != nil {
				state.Status = http.StatusBadGateway
				state.Error = err
				http.Error(w, state.Error.Error(), state.Status)
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
		state.Status = http.StatusNotFound
		state.Error = err
		http.Error(w, state.Error.Error(), state.Status)
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
	first := true
	for {
		bytes, err := openFile.Read((*buf))

		if bytes == 0 {
			break
		}

		if err != nil && err != io.EOF {
			state.Status = http.StatusInternalServerError
			state.Error = err
			http.Error(w, state.Error.Error(), state.Status)
			return
		}

		contentType := ""
		if first {
			contentType = http.DetectContentType((*buf)[:bytes])
			w.Header().Set("Content-Type", contentType)
		}

		bytesWritten, err := w.Write((*buf)[:bytes])

		if err != nil {
			state.Status = http.StatusBadGateway
			state.Error = err
			http.Error(w, state.Error.Error(), state.Status)
			return
		}

		if opts.CacheEnabled {
			cachedEntry.Data = append(cachedEntry.Data, (*buf)[:bytes]...)
			if first {
				cachedEntry.ContentType = contentType
			}
		}

		state.Size += bytesWritten

		first = false
	}

	cachedEntry.IsLoaded = true
}
