package handlers_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tahaa-Dev/go-serve/handlers"
	"github.com/Tahaa-Dev/go-serve/utils"
)

type testResponseWriter struct {
	data    []byte
	status  int
	headers http.Header
}

func (t *testResponseWriter) Header() http.Header {
	return t.headers
}

func (t *testResponseWriter) Write(b []byte) (int, error) {
	t.data = append(t.data, b...)

	return len(b), nil
}

func (t *testResponseWriter) WriteHeader(statusCode int) {
	t.status = statusCode
}

func TestRequestHandlerNoCache(t *testing.T) {
	dir := t.TempDir()
	file, err := os.CreateTemp(dir, "page.html")
	if err != nil {
		t.Error(err.Error())
		return
	}
	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>Test</h1>\n</body>\n</html>")
	_, err = file.Write(data)
	if err != nil {
		t.Error(err.Error())
		return
	}

	cache := utils.NewCache(0)
	state := utils.NewLogState()
	state.CheckAuth = false

	req, err := http.NewRequest("GET", "http://127.0.0.1:8000/"+filepath.Base(file.Name()), nil)
	if err != nil {
		t.Error(err.Error())
		return
	}

	w := testResponseWriter{make([]byte, 0, 1024), http.StatusOK, make(http.Header)}

	handlers.RequestHandler(
		&utils.StateResW{State: &state, W: &w},
		req,
		utils.ReqHandlerOpts{Dir: dir, Cache: &cache},
	)

	if state.Size != len(data) {
		t.Errorf("Unexpected size state: %d", state.Size)
	}
	if state.Status != http.StatusOK {
		t.Errorf("Unexpected status state: %d", state.Status)
	}
	if state.Error != nil {
		t.Error("Unexpected error:", state.Error.Error())
	}

	if !bytes.Equal(w.data, data) {
		t.Errorf("Unexpected data:\n%s", w.data)
	}
	// Content-Type will be application/octet-stream since the extension would be unknown
	if w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Errorf("Unexpected Content-Type header: %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Content-Length") != fmt.Sprintf("%d", len(data)) {
		t.Errorf("Unexpected Content-Length header: %s", w.Header().Get("Content-Length"))
	}
}

func TestRequestHandlerNotCached(t *testing.T) {
	dir := t.TempDir()
	file, err := os.CreateTemp(dir, "page.html")
	if err != nil {
		t.Error(err.Error())
		return
	}
	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>Test</h1>\n</body>\n</html>")
	_, err = file.Write(data)
	if err != nil {
		t.Error(err.Error())
		return
	}

	cache := utils.NewCache(4)
	state := utils.NewLogState()
	state.CheckAuth = false
	filename := filepath.Base(file.Name())

	req, err := http.NewRequest("GET", "http://127.0.0.1:8000/"+filename, nil)
	if err != nil {
		t.Error(err.Error())
		return
	}

	w := testResponseWriter{make([]byte, 0, 1024), http.StatusOK, make(http.Header)}

	handlers.RequestHandler(
		&utils.StateResW{State: &state, W: &w},
		req,
		utils.ReqHandlerOpts{Dir: dir, Cache: &cache},
	)

	if state.Size != len(data) {
		t.Errorf("Unexpected size state: %d", state.Size)
	}
	if state.Status != http.StatusOK {
		t.Errorf("Unexpected status state: %d", state.Status)
	}
	if state.Error != nil {
		t.Error("Unexpected error:", state.Error.Error())
	}

	if !bytes.Equal(w.data, data) {
		t.Errorf("Unexpected data:\n%s", w.data)
	}
	// Content-Type will be application/octet-stream since the extension would be unknown
	if w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Errorf("Unexpected Content-Type header: %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Content-Length") != fmt.Sprintf("%d", len(data)) {
		t.Errorf("Unexpected Content-Length header: %s", w.Header().Get("Content-Length"))
	}

	name := filepath.Clean(req.URL.Path)
	entry := cache.Get(&name)
	if entry.Freq.Load() != 1 {
		t.Errorf("Unexpected entry.Freq: %d", entry.Freq.Load())
	}
	if !bytes.Equal(entry.Data, data) {
		t.Errorf("Unexpected entry.Data:\n%s", entry.Data)
	}
	if entry.ContentType != "application/octet-stream" {
		t.Errorf("Unexpected entry.ContentType: %s", entry.ContentType)
	}
	if !entry.IsLoaded {
		t.Error("Expected entry to be loaded")
	}
}
