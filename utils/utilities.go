package utils

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

var AuthVar string

func init() {
	isSet := false

	if AuthVar, isSet = os.LookupEnv("GO_SERVE_AUTH"); !isSet {
		fmt.Fprintln(
			os.Stderr,
			"Env Var 'GO_SERVE_AUTH' not found.\n • Set Var to a secure auth token for GET /test route authorization",
		)
		os.Exit(1)
	}
}

func LogRequest(
	msg LogMessage,
	ch chan<- LogMessage,
	threshold int,
	logHeader string,
) {
	switch logHeader {
	case "Error":
		if threshold > 400 {
			threshold = 400
		}
	case "Warn":
		if threshold > 300 {
			threshold = 300
		}
	case "Info":
		if threshold > 200 {
			threshold = 200
		}
	default:
	}

	if msg.Status >= threshold {
		ch <- msg
	}
}

func Auth(
	status *int,
	err *error,
	authHeader string,
	w http.ResponseWriter,
) bool {
	if subtle.ConstantTimeCompare(
		[]byte(authHeader),
		[]byte("Bearer "+AuthVar),
	) == 0 {
		errStr := "unauthorized request attempt"
		*status = http.StatusUnauthorized
		*err = errors.New(errStr)

		w.Header().Set("WWW-Authenticate", "Bearer realm=\"Test\"")
		http.Error(w, errStr, *status)

		return false
	}

	return true
}

func LogMiddleware(
	next http.Handler,
	logChan chan<- LogMessage,
	logThreshold int,
	state *LogState,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			LogRequest(
				LogMessage{
					state.StartTime,
					time.Since(state.StartTime),
					req.URL.Path,
					req.Method,
					state.Status,
					state.Size,
					state.Error,
				},
				logChan,
				logThreshold,
				req.Header.Get("Logging"),
			)
		}()

		if state.CheckAuth && !Auth(
			&state.Status,
			&state.Error,
			req.Header.Get("Authorization"),
			w,
		) {
			return
		}

		next.ServeHTTP(w, req)
	})
}
