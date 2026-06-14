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
	rw http.ResponseWriter,
	req *http.Request,
	opts utils.ReqHandlerOpts,
) {
	w := rw.(*utils.StateResW)
	safePath := filepath.Clean(req.URL.Path)
	fullPath := filepath.Join(opts.Dir, safePath)

	if err := os.Remove(fullPath); err != nil {
		w.State.Error = err
		if errors.Is(w.State.Error, os.ErrNotExist) {
			w.State.Status = http.StatusNotFound
		} else {
			w.State.Status = http.StatusInternalServerError
		}
		http.Error(w, w.State.Error.Error(), w.State.Status)
		return
	}

	if opts.Cache.Cap > 0 {
		opts.Cache.Delete(&safePath)
	}

	message := []byte("file deleted successfully")
	w.State.Status = http.StatusOK
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(message)))

	n, err := w.Write(message)
	w.State.Size = n
	if err != nil {
		w.State.Status = http.StatusBadGateway
		w.State.Error = err
		http.Error(w, w.State.Error.Error(), w.State.Status)
		return
	}
}
