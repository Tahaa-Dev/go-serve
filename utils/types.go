package utils

import (
	"net/http"
	"time"
)

type ReqHandlerOpts struct {
	Dir   string
	Cache *Cache
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

func NewLogState(chechAuth bool) LogState {
	return LogState{
		StartTime: time.Now(),
		Status:    http.StatusOK,
		Size:      0,
		Error:     nil,
		CheckAuth: chechAuth,
	}
}
