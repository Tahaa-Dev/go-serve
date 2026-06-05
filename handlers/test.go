package handlers

import (
	"fmt"
	"github.com/Tahaa-Dev/go-serve/utils"
	"net/http"
	"os"
	"time"
)

var testGarbage = make([]byte, 128*1024)

func TestHandler(
	w http.ResponseWriter,
	req *http.Request,
	state *utils.LogState,
) {
	size := 50

	if mb := req.URL.Query().Get("mb"); mb != "" {
		n, err := fmt.Sscanf(mb, "%d", &size)

		if err != nil || n == 0 {
			fmt.Fprintln(os.Stderr, "Error while scanning query string for size")
		}
	}

	rc := http.NewResponseController(w)
	err := rc.SetWriteDeadline(time.Time{})

	if err != nil {
		state.Status = http.StatusHTTPVersionNotSupported
		state.Error = err
		http.Error(w, state.Error.Error(), state.Status)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size*1024*1024))

	for i := 0; i < size*8; i++ {
		n, err := w.Write(testGarbage)

		if err != nil {
			state.Status = http.StatusBadGateway
			state.Error = err
			http.Error(w, state.Error.Error(), state.Status)
			break
		}

		state.Size += n
	}
}
