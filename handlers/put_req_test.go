package handlers_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tahaa-Dev/go-serve/handlers"
	"github.com/Tahaa-Dev/go-serve/utils"
)

func TestPutRequestHandlerErrorless(t *testing.T) {
	dir := t.TempDir()
	file, err := os.CreateTemp(dir, "page")
	if err != nil {
		t.Error(err.Error())
		return
	}

	oldData := []byte("old data2")
	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>Test</h1>\n</body>\n</html>")
	_, err = file.Write(oldData)
	if err != nil {
		t.Error(err.Error())
		return
	}
	err = file.Close()
	if err != nil {
		t.Error(err.Error())
		return
	}

	buf := bytes.NewBuffer(data)

	w := testResponseWriter{make([]byte, 0, 1024), http.StatusOK, make(http.Header)}
	req, err := http.NewRequest("PUT", "http://localhost:8000/"+filepath.Base(file.Name()), buf)
	if err != nil {
		t.Error(err.Error())
		return
	}

	state := utils.NewLogState()
	cache := utils.NewCache(4)
	name := filepath.Clean(req.URL.Path)
	cache.Add(&name, oldData, cache.Get(&name))

	handlers.PutRequestHandler(
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

	entry := cache.Get(&name)
	if entry.Freq.Load() != 2 {
		t.Errorf("Unexpected entry.Freq: %d", entry.Freq.Load())
	}
	if entry.ContentType != "application/octet-stream" {
		t.Errorf("Unexpected entry.ContentType: %s", entry.ContentType)
	}
	if !bytes.Equal(entry.Data, data) {
		t.Errorf("Unexpected entry.Data:\n %s", entry.Data)
	}
	if cache.MinFreq.Load() != 2 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq.Load())
	}
	if _, exists := cache.LFUBuckets[entry.Freq.Load()].Bucket[name]; !exists {
		t.Error("Unexpected LFUBuckets")
	}

	newData := make([]byte, 1024)
	file, err = os.Open(file.Name())
	if err != nil {
		t.Error(err.Error())
		return
	}
	if n, err := file.Read(newData); err != nil || !bytes.Equal(newData[:n], data) {
		t.Errorf("Unexpected file.Data:\n%s", newData[:n])
	}
}

func TestPutRequestHandlerError(t *testing.T) {
	dir := t.TempDir()

	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>Test</h1>\n</body>\n</html>")
	buf := bytes.NewBuffer(data)

	w := testResponseWriter{make([]byte, 0, 1024), http.StatusOK, make(http.Header)}
	req, err := http.NewRequest("PUT", "http://localhost:8000/page.html", buf)
	if err != nil {
		t.Error(err.Error())
		return
	}
	state := utils.NewLogState()
	cache := utils.NewCache(4)

	handlers.PutRequestHandler(
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
