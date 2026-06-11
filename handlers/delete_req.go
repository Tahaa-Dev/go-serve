package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Tahaa-Dev/go-serve/utils"
)

func DeleteRequestHandler(
	w http.ResponseWriter,
	req *http.Request,
	state *utils.LogState,
	opts utils.ReqHandlerOpts,
) {
	safePath := filepath.Clean(req.URL.Path)
	fullPath := filepath.Join(opts.Dir, safePath)

	if err := os.Remove(fullPath); err != nil {
		state.Error = err
		if errors.Is(state.Error, os.ErrNotExist) {
			state.Status = http.StatusNotFound
		} else {
			state.Status = http.StatusInternalServerError
		}
		http.Error(w, state.Error.Error(), state.Status)
		return
	}

	if opts.Cache.Cap > 0 {
		opts.Cache.Delete(&safePath)
	}

	message := []byte("file deleted successfully")
	state.Status = http.StatusOK
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(message)))

	n, err := w.Write(message)
	state.Size = n
	if err != nil {
		state.Status = http.StatusBadGateway
		state.Error = err
		http.Error(w, state.Error.Error(), state.Status)
		return
	}
}
