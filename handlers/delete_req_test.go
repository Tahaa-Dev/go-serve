package handlers_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tahaa-Dev/go-serve/handlers"
	"github.com/Tahaa-Dev/go-serve/utils"
)

func TestDeleteRequestHandlerExists(t *testing.T) {
	dir := t.TempDir()
	file, err := os.CreateTemp(dir, "page")
	if err != nil {
		t.Error(err.Error())
		return
	}

	err = file.Close()
	if err != nil {
		t.Error(err.Error())
		return
	}

	w := testResponseWriter{make([]byte, 0, 1024), http.StatusOK, make(http.Header)}
	req, err := http.NewRequest("DELETE", "http://localhost:8000/"+filepath.Base(file.Name()), nil)
	if err != nil {
		t.Error(err.Error())
		return
	}

	state := utils.NewLogState()
	cache := utils.NewCache(4)
	name := filepath.Clean(req.URL.Path)
	cache.Add(&name, []byte{0}, cache.Get(&name))

	handlers.DeleteRequestHandler(
		&utils.StateResW{State: &state, W: &w},
		req,
		utils.ReqHandlerOpts{Dir: dir, Cache: &cache},
	)

	if state.Error != nil {
		t.Errorf("Unexpected error:\n %s", state.Error.Error())
	}
	if state.Status != http.StatusOK || w.status != http.StatusOK {
		t.Errorf("Unexpected HTTP status: %d", state.Status)
	}

	filename := filepath.Clean(req.URL.Path)
	if cache.Get(&filename).Data != nil {
		t.Error("Expected csche to not contain file:", filename)
	}
	if cache.MinFreq.Load() != 0 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq.Load())
	}
	if cache.Size.Load() != 0 {
		t.Errorf("Unexpected cache.Size: %d", cache.Size.Load())
	}

	if _, err = os.Stat(file.Name()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Unexpected file open error: %s", err.Error())
	}
}

func TestDeleteRequestHandlerNotExists(t *testing.T) {
	dir := t.TempDir()

	w := testResponseWriter{make([]byte, 0, 1024), http.StatusOK, make(http.Header)}
	req, err := http.NewRequest("DELETE", "http://localhost:8000/page.html", nil)
	if err != nil {
		t.Error(err.Error())
		return
	}
	state := utils.NewLogState()
	cache := utils.NewCache(4)

	handlers.DeleteRequestHandler(
		&utils.StateResW{State: &state, W: &w},
		req,
		utils.ReqHandlerOpts{Dir: dir, Cache: &cache},
	)

	if state.Error == nil {
		t.Error("Expected error")
	}
	if state.Status != http.StatusNotFound || w.status != http.StatusNotFound {
		t.Error("Expected 404 Not Found status, found:", state.Status)
	}
}
