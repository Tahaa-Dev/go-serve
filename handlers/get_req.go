package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Tahaa-Dev/go-serve/utils"
)

var bufPool = utils.NewPool()

func RequestHandler(
	w *utils.StateResW,
	req *http.Request,
	opts utils.ReqHandlerOpts,
) {
	var cachedEntry *utils.CacheEntry

	cacheEnabled := opts.Cache.Cap > 0

	safePath := filepath.Clean(req.URL.Path)

	if cacheEnabled {
		cachedFile := opts.Cache.Get(&safePath)

		checkCache := func() {
			contentType := ""

			if cachedFile.ContentType == "NOT ADDED" {
				contentType = utils.TypeByExtension(filepath.Ext(safePath))
				if contentType == "" {
					contentType = "application/octet-stream"
				}
			} else {
				contentType = cachedFile.ContentType
			}
			cachedFile.ContentType = contentType

			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(cachedFile.Data)))
			// #nosec G705 -- intentional file server design
			length, err := w.Write(cachedFile.Data)
			w.State.Size = length

			if err != nil {
				w.State.Status = http.StatusBadGateway
				w.State.Error = err
				http.Error(w, w.State.Error.Error(), w.State.Status)
			}
		}

		cachedFile.Mu.RLock()
		if cachedFile.IsLoaded {
			checkCache()
			cachedFile.Mu.RUnlock()
			return
		}
		cachedFile.Mu.RUnlock()

		cachedFile.Mu.Lock()
		cachedEntry = cachedFile
		defer func() {
			cachedEntry.Mu.Unlock()
			if w.State.Error != nil {
				cachedEntry.Data = nil
			}
		}()

		// double check if another goroutine built the cache while
		// this goroutine was waiting for the write lock
		if cachedEntry.IsLoaded {
			checkCache()
			return
		}
	}

	fullPath := filepath.Join(opts.Dir, safePath)
	openFile, err := os.OpenFile(fullPath, os.O_RDONLY, 0400)
	if err != nil {
		w.State.Error = err
		if errors.Is(w.State.Error, os.ErrNotExist) {
			w.State.Status = http.StatusNotFound
		} else {
			w.State.Status = http.StatusInternalServerError
		}
		http.Error(w, w.State.Error.Error(), w.State.Status)
		return
	}
	defer func() {
		_ = openFile.Close()
	}()

	fileInfo, err := openFile.Stat()
	if err != nil {
		w.State.Error = err
		w.State.Status = http.StatusInternalServerError
		http.Error(w, w.State.Error.Error(), w.State.Status)
		return
	}

	if fileInfo.IsDir() {
		_ = openFile.Close()
		fullPath = filepath.Join(fullPath, opts.Index)

		openFile, err = os.OpenFile(fullPath, os.O_RDONLY, 0400)
		if err != nil {
			w.State.Error = err
			if errors.Is(w.State.Error, os.ErrNotExist) {
				w.State.Status = http.StatusNotFound
			} else {
				w.State.Status = http.StatusInternalServerError
			}
			http.Error(w, w.State.Error.Error(), w.State.Status)
			return
		}

		fileInfo, err = openFile.Stat()
		if err != nil {
			w.State.Error = err
			w.State.Status = http.StatusInternalServerError
			http.Error(w, w.State.Error.Error(), w.State.Status)
			return
		}
	}
	fileSizeTmp := fileInfo.Size()
	fileSize := uint64(0)
	if fileSizeTmp < 0 {
		fileSize = uint64(fileSizeTmp * -1)
	} else {
		fileSize = uint64(fileSizeTmp)
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))
	cacheEnabled = cacheEnabled && (fileSize < uint64(opts.MaxEntrySize*1024*1024))

	contentType := utils.TypeByExtension(filepath.Ext(fullPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	if cacheEnabled {
		cachedEntry.ContentType = contentType
	}

	idx, buf := bufPool.Get()
	defer bufPool.Put(idx)
	for {
		bytesRead, err := openFile.Read(buf[:])

		if bytesRead == 0 {
			break
		}

		if err != nil && !errors.Is(err, io.EOF) {
			w.State.Status = http.StatusInternalServerError
			w.State.Error = err
			return
		}

		bytesWritten, err := w.Write(buf[:bytesRead])

		w.State.Size += bytesWritten

		if err != nil {
			w.State.Status = http.StatusBadGateway
			w.State.Error = err
			return
		}

		if cacheEnabled {
			opts.Cache.Add(&safePath, buf[:bytesRead], cachedEntry)
		}
	}

	if cacheEnabled {
		cachedEntry.IsLoaded = true
	}
}
