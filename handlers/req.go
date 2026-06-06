package handlers

import (
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/Tahaa-Dev/go-serve/utils"
)

var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 128*1024)
		return &buf
	},
}

func RequestHandler(
	w http.ResponseWriter,
	req *http.Request,
	opts utils.ReqHandlerOpts,
	state *utils.LogState,
) {
	var cachedEntry *utils.CacheEntry

	safePath := filepath.Clean(req.URL.Path)

	w.Header().Set("X-Content-Type-Options", "nosniff")

	if opts.Cache.Cap > 0 {
		cachedFile := opts.Cache.Get(&safePath)

		checkCache := func() {
			contentType := ""

			if cachedFile.ContentType == "NOT ADDED" {
				contentType = mime.TypeByExtension(filepath.Ext(safePath))
				if contentType == "" {
					contentType = "application/octet-stream"
				}
			} else {
				contentType = cachedFile.ContentType
			}
			cachedFile.ContentType = contentType

			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(cachedFile.Data)))
			// #nosec G705 -- intentional file server design
			length, err := w.Write(cachedFile.Data)
			state.Size = length

			if err != nil {
				state.Status = http.StatusBadGateway
				state.Error = err
				http.Error(w, state.Error.Error(), state.Status)
			}
		}

		if cachedFile.IsLoaded {
			cachedFile.Mu.RLock()
			checkCache()
			cachedFile.Mu.RUnlock()
			return
		}

		cachedFile.Mu.Lock()
		cachedEntry = cachedFile
		defer cachedEntry.Mu.Unlock()

		// double check if another goroutine built the cache while
		// this goroutine was waiting for the write lock
		if cachedEntry.IsLoaded {
			checkCache()
			return
		}
	}

	contentType := mime.TypeByExtension(filepath.Ext(safePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	fullPath := filepath.Join(opts.Dir, safePath)
	// #nosec G304 -- path is sanitized before cache check
	openFile, err := os.OpenFile(fullPath, os.O_RDONLY, 0400)
	if err != nil {
		state.Status = http.StatusNotFound
		state.Error = err
		http.Error(w, state.Error.Error(), state.Status)
		return
	}

	defer func() {
		err := openFile.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to close file:", fullPath)
		}
	}()

	fileInfo, err := openFile.Stat()
	if err != nil {
		state.Error = err
		state.Status = http.StatusInternalServerError
		http.Error(w, state.Error.Error(), state.Status)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	if fileInfo.IsDir() {
		contentType = "text/html"
		w.Header().Set("Content-Type", contentType)
		paths := ""

		err := filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				path = filepath.Clean(path)[len(opts.Dir)+1:]
				paths += fmt.Sprintf("\n<li><a href=\"%s\">%s</a></li>", path, path)
			}

			return nil
		})

		if err != nil {
			state.Error = err
			state.Status = http.StatusInternalServerError
			http.Error(w, state.Error.Error(), state.Status)
			return
		}

		// allocates a new array since we can't use the pool as formatting can grow the array
		// and directory listing requests are very rare as well
		dirListing := make([]byte, 0, 4*1024)
		dirListing = fmt.Appendf(
			dirListing,
			"<!DOCTYPE HTML>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<title>Directory "+
				"listing for %s/</title>\n</head>\n<body>\n<h1>Directory listing for %s/</h1>"+
				"\n<hr>\n<ul>%s\n</ul>\n<hr>\n</body>\n</html>",
			fullPath[len(opts.Dir):],
			fullPath[len(opts.Dir):],
			paths,
		)

		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(dirListing)))

		// #nosec G705 -- intentional file server design as directory listing is guaranteed to be valid HTML
		bytesWritten, err := w.Write(dirListing)

		state.Size += bytesWritten

		if err != nil {
			state.Status = http.StatusBadGateway
			state.Error = err
			http.Error(w, state.Error.Error(), state.Status)
			return
		}

		if opts.Cache.Cap > 0 {
			opts.Cache.Add(&safePath, dirListing, cachedEntry)
			cachedEntry.ContentType = contentType
		}

		cachedEntry.IsLoaded = true
		return
	}

	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	first := true
	for {
		bytesRead, err := openFile.Read((*buf))

		if bytesRead == 0 {
			break
		}

		if err != nil && err != io.EOF {
			state.Status = http.StatusInternalServerError
			state.Error = err
			http.Error(w, state.Error.Error(), state.Status)
			return
		}

		if first {
			w.Header().Set("Content-Type", contentType)
		}

		bytesWritten, err := w.Write((*buf)[:bytesRead])

		state.Size += bytesWritten

		if err != nil {
			state.Status = http.StatusBadGateway
			state.Error = err
			http.Error(w, state.Error.Error(), state.Status)
			return
		}

		if opts.Cache.Cap > 0 {
			opts.Cache.Add(&safePath, (*buf)[:bytesRead], cachedEntry)
			cachedEntry.ContentType = contentType
		}

		first = false
	}

	cachedEntry.IsLoaded = true
}
