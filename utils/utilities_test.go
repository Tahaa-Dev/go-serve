package utils_test

import (
	"net/http"
	"os"
	"testing"
	"time"

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

func TestLogRequest(t *testing.T) {
	msg := utils.LogMessage{
		StartTime: time.Now(),
		Duration:  time.Millisecond * 3,
		URL:       "/page.html",
		Method:    "GET",
		Status:    http.StatusOK,
		Size:      27000,
		Error:     nil,
	}

	ch := make(chan utils.LogMessage, 2)

	utils.LogRequest(msg, ch, 300, "Info")
	utils.LogRequest(msg, ch, 200, "Warn")
	close(ch)

	i := 0

	for range ch {
		i++
	}

	if i != 2 {
		t.Error("Expected to get 2 messages, but only got", i)
	}
}

func TestAuthAllowed(t *testing.T) {
	status := http.StatusOK
	var err error
	w := testResponseWriter{make([]byte, 0), http.StatusOK, make(map[string][]string)}

	res := utils.Auth(&status, &err, "Bearer "+os.Getenv("GO_SERVE_AUTH"), &w, "Test")

	if !res || err != nil || status != http.StatusOK || w.status != http.StatusOK {
		t.Error("Expected Auth() to authorize request successfully")
	}
}

func TestAuthForbidden(t *testing.T) {
	status := http.StatusOK
	var err error
	w := testResponseWriter{make([]byte, 0), http.StatusOK, make(map[string][]string)}

	res := utils.Auth(&status, &err, "", &w, "Test")

	if res || status != http.StatusUnauthorized || w.status != http.StatusUnauthorized ||
		err.Error() != "unauthorized request attempt" ||
		w.Header().Get("WWW-Authenticate") != "Bearer realm=\"Test\"" {
		t.Error("Expected Auth() to not authorize request")
	}
}
