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
	w http.ResponseWriter,
	req *http.Request,
	state *utils.LogState,
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
		state.Error = err
		if errors.Is(state.Error, os.ErrExist) {
			state.Status = http.StatusConflict
		} else {
			state.Status = http.StatusInternalServerError
		}
		http.Error(w, state.Error.Error(), state.Status)
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

	message := []byte("file created successfully")
	state.Status = http.StatusCreated
	w.Header().Set("Location", safePath)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(message)))
	w.WriteHeader(state.Status)

	idx, buf := bufPool.Get()
	defer bufPool.Put(idx)

	var cachedEntry *utils.CacheEntry
	if opts.Cache.Cap > 0 {
		cachedEntry = opts.Cache.Get(&safePath)
		cachedEntry.Mu.Lock()
		defer cachedEntry.Mu.Unlock()
	}

	for {
		bytesRead, err := req.Body.Read(buf[:])

		if bytesRead == 0 {
			break
		}

		if err != nil && !errors.Is(err, io.EOF) {
			state.Status = http.StatusInternalServerError
			state.Error = err
			http.Error(w, state.Error.Error(), state.Status)
			return
		}

		_, err = file.Write(buf[:bytesRead])
		if err != nil {
			state.Status = http.StatusInternalServerError
			state.Error = err
			http.Error(w, state.Error.Error(), state.Status)
			return
		}

		if opts.Cache.Cap > 0 {
			opts.Cache.Add(&safePath, buf[:bytesRead], cachedEntry)
		}
	}

	n, err := w.Write(message)
	state.Size = n
	if err != nil {
		state.Status = http.StatusBadGateway
		state.Error = err
		http.Error(w, state.Error.Error(), state.Status)
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
