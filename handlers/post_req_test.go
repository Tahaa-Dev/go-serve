package handlers_test

import (
	"bytes"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/Tahaa-Dev/go-serve/handlers"
	"github.com/Tahaa-Dev/go-serve/utils"
)

func TestPostRequestHandlerErrorless(t *testing.T) {
	dir := t.TempDir()

	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>Test</h1>\n</body>\n</html>")
	buf := bytes.NewBuffer(data)

	w := testResponseWriter{make([]byte, 0, 1024), http.StatusOK, make(http.Header)}
	req, err := http.NewRequest("POST", "http://localhost:8000/page.html", buf)
	if err != nil {
		t.Error(err.Error())
		return
	}
	state := utils.NewLogState(true)
	cache := utils.NewCache(4)

	handlers.PostRequestHandler(&w, req, &state, utils.ReqHandlerOpts{Dir: dir, Cache: &cache})

	if state.Error != nil {
		t.Errorf("Unexpected error:\n %s", state.Error.Error())
	}
	if state.Status != http.StatusCreated || w.status != http.StatusCreated {
		t.Errorf("Unexpected HTTP status: %d", state.Status)
	}

	name := filepath.Clean(req.URL.Path)
	entry := cache.Get(&name)
	if entry.Freq != 1 {
		t.Errorf("Unexpected entry.Freq: %d", entry.Freq)
	}
	if entry.ContentType != "text/html" {
		t.Errorf("Unexpected entry.ContentType: %s", entry.ContentType)
	}
	if !bytes.Equal(entry.Data, data) {
		t.Errorf("Unexpected entry.Data:\n %s", entry.Data)
	}
	if cache.MinFreq != 1 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq)
	}
	if idx := bytes.Index(cache.LFUBuckets[1], []byte(name)); idx != 0 {
		t.Errorf("Unexpected LFUBuckets[1] index: %d", idx)
	}
}
