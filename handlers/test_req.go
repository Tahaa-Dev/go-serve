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
	rw http.ResponseWriter,
	req *http.Request,
) {
	size := 50
	w := rw.(*utils.StateResW)

	if mb := req.URL.Query().Get("mb"); mb != "" {
		n, err := fmt.Sscanf(mb, "%d", &size)

		if err != nil || n == 0 {
			fmt.Fprintln(os.Stderr, "Error while scanning query string for size")
		}
	}

	rc := http.NewResponseController(w)
	err := rc.SetWriteDeadline(time.Time{})

	if err != nil {
		w.State.Status = http.StatusHTTPVersionNotSupported
		w.State.Error = err
		http.Error(w, w.State.Error.Error(), w.State.Status)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size*1024*1024))

	for i := 0; i < size*8; i++ {
		n, err := w.Write(testGarbage)

		if err != nil {
			w.State.Status = http.StatusBadGateway
			w.State.Error = err
			http.Error(w, w.State.Error.Error(), w.State.Status)
			break
		}

		w.State.Size += n
	}
}
