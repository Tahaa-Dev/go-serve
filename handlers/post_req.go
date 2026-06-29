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

func PostRequestHandler(
	w *utils.StateResW,
	req *http.Request,
	opts utils.ReqHandlerOpts,
) {
	safePath := filepath.Clean(req.URL.Path)
	fullPath := filepath.Join(opts.Dir, safePath)
	file, err := os.OpenFile(
		fullPath,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0600,
	)

	if err != nil {
		w.State.Error = err
		if errors.Is(w.State.Error, os.ErrExist) {
			w.State.Status = http.StatusConflict
		} else {
			w.State.Status = http.StatusInternalServerError
		}
		http.Error(w, w.State.Error.Error(), w.State.Status)
		return
	}

	defer func() {
		err := file.Close()
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"Error while closing file: %s\n • Error Message: %s",
				fullPath,
				err.Error(),
			)
		}
	}()

	fileInfo, err := file.Stat()
	if err == nil && fileInfo.IsDir() {
		w.State.Error = fmt.Errorf("bad request: path '%s' is a directory", fullPath)
		w.State.Status = http.StatusBadRequest
		http.Error(w, w.State.Error.Error(), w.State.Status)
		return
	}

	message := []byte("file created successfully")
	w.State.Status = http.StatusCreated
	w.Header().Set("Location", safePath)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(message)))
	w.WriteHeader(w.State.Status)

	idx, buf := bufPool.Get()
	defer bufPool.Put(idx)

	var cachedEntry *utils.CacheEntry
	if opts.Cache.Cap > 0 {
		cachedEntry = opts.Cache.Get(&safePath)
		cachedEntry.Mu.Lock()
		defer cachedEntry.Mu.Unlock()
		defer func() {
			if w.State.Error != nil {
				cachedEntry.Data = nil
			}
		}()
	}

	for {
		bytesRead, err := req.Body.Read(buf[:])

		if bytesRead == 0 {
			break
		}

		if err != nil && !errors.Is(err, io.EOF) {
			w.State.Status = http.StatusBadGateway
			w.State.Error = err
			http.Error(w, w.State.Error.Error(), w.State.Status)
			return
		}

		_, err = file.Write(buf[:bytesRead])
		if err != nil {
			w.State.Status = http.StatusInternalServerError
			w.State.Error = err
			http.Error(w, w.State.Error.Error(), w.State.Status)
			return
		}

		if opts.Cache.Cap > 0 {
			opts.Cache.Add(&safePath, buf[:bytesRead], cachedEntry)
		}
	}

	n, err := w.Write(message)
	w.State.Size = n
	if err != nil {
		w.State.Status = http.StatusBadGateway
		w.State.Error = err
		http.Error(w, w.State.Error.Error(), w.State.Status)
		return
	}

	if opts.Cache.Cap > 0 {
		cachedEntry.ContentType = utils.TypeByExtension(filepath.Ext(safePath))
		if cachedEntry.ContentType == "" {
			cachedEntry.ContentType = "application/octet-stream"
		}
		cachedEntry.IsLoaded = true
	}
}
