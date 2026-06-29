package utils

import (
	"net/http"
	"time"
)

type ReqHandlerOpts struct {
	Dir   string
	Cache *Cache
	Index string
}

type LogMessage struct {
	StartTime time.Time
	Duration  time.Duration
	URL       string
	Method    string
	Status    int
	Size      int
	Error     error
}

type LogState struct {
	StartTime time.Time
	Status    int
	Size      int
	Error     error
	CheckAuth bool
}

func NewLogState() LogState {
	return LogState{
		StartTime: time.Now(),
		Status:    http.StatusOK,
		Size:      0,
		Error:     nil,
		CheckAuth: true,
	}
}

type StateResW struct {
	State *LogState
	W     http.ResponseWriter
}

func (srw *StateResW) Header() http.Header {
	return srw.W.Header()
}

func (srw *StateResW) Write(b []byte) (int, error) {
	return srw.W.Write(b)
}

func (srw *StateResW) WriteHeader(statusCode int) {
	srw.W.WriteHeader(statusCode)
}
