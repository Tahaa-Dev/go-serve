package utils

import (
	"bufio"
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
			"Env Var 'GO_SERVE_AUTH' not found.\n • Set Var to a secure auth token for GET /test route, POST / route, and pprof diagnsotics authorization",
		)
		os.Exit(1)
	}
}

func Auth(
	status *int,
	err *error,
	authHeader string,
	w http.ResponseWriter,
	bearerRealm string,
) bool {
	if subtle.ConstantTimeCompare(
		[]byte(authHeader),
		[]byte("Bearer "+AuthVar),
	) == 0 {
		errStr := "unauthorized request attempt"
		*status = http.StatusUnauthorized
		*err = errors.New(errStr)

		w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer realm=\"%s\"", bearerRealm))
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
	bearerRealm string,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if state.Status >= logThreshold {
				logChan <- LogMessage{
					state.StartTime,
					time.Since(state.StartTime),
					req.URL.Path,
					req.Method,
					state.Status,
					state.Size,
					state.Error,
				}
			}
		}()

		if state.CheckAuth && !Auth(
			&state.Status,
			&state.Error,
			req.Header.Get("Authorization"),
			w,
			bearerRealm,
		) {
			return
		}

		next.ServeHTTP(w, req)
	})
}

func WriteLogs(logChan chan LogMessage, logBuf *bufio.Writer, maxAge int64, idleTime int64) {
	idleDuration := time.Duration(idleTime) * time.Millisecond
	idleTicker := time.NewTicker(idleDuration)
	defer idleTicker.Stop()

	maxDuration := time.Duration(maxAge) * time.Second
	maxAgeTicker := time.NewTicker(maxDuration)
	defer maxAgeTicker.Stop()

LogLoop:
	for {
		select {
		case msg, ok := <-logChan:
			if !ok {
				break LogLoop
			}

			errStr := "\n"
			if msg.Error != nil {
				errStr = fmt.Sprintf(" | Error: %s\n", msg.Error)
			}

			_, err := fmt.Fprintf(
				logBuf,
				"[%s] %s %s: Status: %d | Size: %d | Time: %s%s",
				msg.StartTime.Format("15:04:05"),
				msg.Method,
				msg.URL,
				msg.Status,
				msg.Size,
				msg.Duration,
				errStr,
			)

			if err != nil {
				fmt.Fprintln(
					os.Stderr,
					"Failed to write log at:",
					msg.StartTime.Local().Format("15:04:05"),
				)
			}
			idleTicker.Reset(idleDuration)

		case <-idleTicker.C:
			err := logBuf.Flush()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to flush logs")
			}

			idleTicker.Reset(idleDuration)

		case <-maxAgeTicker.C:
			err := logBuf.Flush()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to flush logs")
			}
		}
	}

	err := logBuf.Flush()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to flush logs")
	}
}
