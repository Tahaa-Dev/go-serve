package utils_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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

func TestLogMiddlewareErrorless(t *testing.T) {
	state := utils.NewLogState(true)
	ch := make(chan utils.LogMessage, 1)
	resp := testResponseWriter{make([]byte, 0), http.StatusOK, make(map[string][]string)}

	utils.LogMiddleware(
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}),
		ch,
		200,
		&state,
		"Test",
	).ServeHTTP(&resp, &http.Request{
		Header: map[string][]string{
			"Authorization": {"Bearer " + os.Getenv("GO_SERVE_AUTH")},
		},
		URL:    &url.URL{Path: "/"},
		Method: "GET",
	})

	close(ch)

	i := 0
	for range ch {
		i++
	}
	if resp.status != http.StatusOK || state.Status != http.StatusOK || state.Error != nil ||
		i != 1 ||
		resp.Header().Get("WWW-Authenticate") == "Bearer realm=\"Test\"" {
		t.Error("Expected LogMiddleware to send one log through the channel without mutating state")
	}
}

func TestLogMiddlewareAuthError(t *testing.T) {
	state := utils.NewLogState(true)
	ch := make(chan utils.LogMessage, 1)
	resp := testResponseWriter{make([]byte, 0), http.StatusOK, make(map[string][]string)}

	utils.LogMiddleware(
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}),
		ch,
		200,
		&state,
		"Realm",
	).ServeHTTP(&resp, &http.Request{
		Header: map[string][]string{},
		URL:    &url.URL{Path: "/"},
		Method: "GET",
	})

	close(ch)

	i := 0
	for range ch {
		i++
	}

	if resp.status != http.StatusUnauthorized || state.Status != http.StatusUnauthorized ||
		state.Error.Error() != "unauthorized request attempt" ||
		i != 1 ||
		resp.Header().Get("WWW-Authenticate") != "Bearer realm=\"Realm\"" {
		t.Error("Expected LogMiddleware to send one log through the channel and mutate state")
	}
}

func TestLogMiddlewareStateMutation(t *testing.T) {
	state := utils.NewLogState(false)
	ch := make(chan utils.LogMessage, 1)
	resp := testResponseWriter{make([]byte, 0), http.StatusOK, make(map[string][]string)}

	utils.LogMiddleware(
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			state.Status = http.StatusInternalServerError
		}),
		ch,
		200,
		&state,
		"Test",
	).ServeHTTP(&resp, &http.Request{
		URL:    &url.URL{Path: "/"},
		Method: "GET",
	})

	close(ch)

	msg := <-ch

	if msg.Status != http.StatusInternalServerError ||
		state.Status != http.StatusInternalServerError ||
		resp.Header().Get("WWW-Authenticate") == "Bearer realm=\"Test\"" {
		t.Error("Expected LogMiddleware to send one log through the channel with mutated state")
	}
}

func TestWriteLogsIdle(t *testing.T) {
	ch := make(chan utils.LogMessage, 1)
	b := bytes.NewBuffer([]byte{})
	buf := bufio.NewWriterSize(b, 1024*1024)

	testTime := time.Now()
	pageSize := (6 * 1024 * 1024) + 79

	go utils.WriteLogs(ch, buf, 1, 50)

	utils.LogRequest(
		utils.LogMessage{
			StartTime: testTime,
			Duration:  1 * time.Second,
			Method:    "GET",
			URL:       "/page.html",
			Status:    http.StatusOK,
			Size:      pageSize,
			Error:     nil,
		},
		ch,
		200,
		"Info",
	)
	utils.LogRequest(
		utils.LogMessage{
			StartTime: testTime,
			Duration:  25 * time.Millisecond,
			Method:    "GET",
			URL:       "/",
			Status:    http.StatusInternalServerError,
			Size:      0,
			Error:     errors.New("TEST"),
		},
		ch,
		200,
		"Info",
	)

	time.Sleep(70 * time.Millisecond)
	close(ch)

	if buffer := b.Bytes(); !bytes.Equal(
		buffer,
		fmt.Appendf(
			[]byte{},
			"[%s] GET /page.html: Status: 200 | Size: %d | Time: 1s"+
				"\n[%s] GET /: Status: 500 | Size: 0 | Time: 25ms | Error: TEST\n",
			testTime.Format("15:04:05"),
			pageSize,
			testTime.Format("15:04:05"),
		),
	) {
		t.Errorf("Enexpected log output:\n%s", buffer)
	}
}

func TestWriteLogsActive(t *testing.T) {
	ch := make(chan utils.LogMessage, 1)
	b := bytes.NewBuffer([]byte{})
	buf := bufio.NewWriterSize(b, 1024*1024)

	go utils.WriteLogs(ch, buf, 1, 25000)

	testTime := time.Now()
	pageSize := (6 * 1024 * 1024) + 79

	utils.LogRequest(
		utils.LogMessage{
			StartTime: testTime,
			Duration:  1 * time.Second,
			Method:    "GET",
			URL:       "/page.html",
			Status:    http.StatusOK,
			Size:      pageSize,
			Error:     nil,
		},
		ch,
		200,
		"Info",
	)
	utils.LogRequest(
		utils.LogMessage{
			StartTime: testTime,
			Duration:  25 * time.Millisecond,
			Method:    "GET",
			URL:       "/",
			Status:    http.StatusInternalServerError,
			Size:      0,
			Error:     errors.New("TEST"),
		},
		ch,
		200,
		"Info",
	)

	time.Sleep(1100 * time.Millisecond)
	close(ch)

	if buffer := b.Bytes(); !bytes.Equal(
		buffer,
		fmt.Appendf(
			[]byte{},
			"[%s] GET /page.html: Status: 200 | Size: %d | Time: 1s"+
				"\n[%s] GET /: Status: 500 | Size: 0 | Time: 25ms | Error: TEST\n",
			testTime.Format("15:04:05"),
			pageSize,
			testTime.Format("15:04:05"),
		),
	) {
		t.Errorf("Enexpected log output:\n%s", buffer)
	}
}
