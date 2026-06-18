package handlers

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Tahaa-Dev/go-serve/utils"
)

var bufPool = utils.NewPool()

func RequestHandler(
	rw http.ResponseWriter,
	req *http.Request,
	opts utils.ReqHandlerOpts,
) {
	var cachedEntry *utils.CacheEntry
	w := rw.(*utils.StateResW)

	safePath := filepath.Clean(req.URL.Path)

	if opts.Cache.Cap > 0 {
		cachedFile := opts.Cache.Get(&safePath)

		checkCache := func() {
			contentType := ""

			if cachedFile.ContentType == "NOT ADDED" {
				contentType = utils.TypeByExtension(filepath.Ext(safePath))
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
			w.State.Size = length

			if err != nil {
				w.State.Status = http.StatusBadGateway
				w.State.Error = err
				http.Error(w, w.State.Error.Error(), w.State.Status)
			}
		}

		cachedFile.Mu.RLock()
		if cachedFile.IsLoaded {
			checkCache()
			cachedFile.Mu.RUnlock()
			return
		}
		cachedFile.Mu.RUnlock()

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

	contentType := utils.TypeByExtension(filepath.Ext(safePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	fullPath := filepath.Join(opts.Dir, safePath)
	openFile, err := os.OpenFile(fullPath, os.O_RDONLY, 0400)
	if err != nil {
		w.State.Error = err
		if errors.Is(w.State.Error, os.ErrNotExist) {
			w.State.Status = http.StatusNotFound
		} else {
			w.State.Status = http.StatusInternalServerError
		}
		http.Error(w, w.State.Error.Error(), w.State.Status)
		return
	}

	defer func() {
		err := openFile.Close()
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"Error while closing file: %s\n • Error Message: %s",
				fullPath,
				err.Error(),
			)
		}
	}()

	fileInfo, err := openFile.Stat()
	if err != nil {
		w.State.Error = err
		w.State.Status = http.StatusInternalServerError
		http.Error(w, w.State.Error.Error(), w.State.Status)
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
			w.State.Error = err
			w.State.Status = http.StatusInternalServerError
			http.Error(w, w.State.Error.Error(), w.State.Status)
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

		w.State.Size += bytesWritten

		if err != nil {
			w.State.Status = http.StatusBadGateway
			w.State.Error = err
			http.Error(w, w.State.Error.Error(), w.State.Status)
			return
		}

		if opts.Cache.Cap > 0 {
			opts.Cache.Add(&safePath, dirListing, cachedEntry)
			cachedEntry.ContentType = contentType
		}

		cachedEntry.IsLoaded = true
		return
	}

	idx, buf := bufPool.Get()
	defer bufPool.Put(idx)
	first := true
	for {
		bytesRead, err := openFile.Read(buf[:])

		if bytesRead == 0 {
			break
		}

		if err != nil && !errors.Is(err, io.EOF) {
			w.State.Status = http.StatusInternalServerError
			w.State.Error = err
			http.Error(w, w.State.Error.Error(), w.State.Status)
			if opts.Cache.Cap > 0 {
				opts.Cache.Delete(&safePath)
			}
			return
		}

		if first {
			w.Header().Set("Content-Type", contentType)
			if opts.Cache.Cap > 0 {
				cachedEntry.ContentType = contentType
			}
		}

		bytesWritten, err := w.Write(buf[:bytesRead])

		w.State.Size += bytesWritten

		if err != nil {
			w.State.Status = http.StatusBadGateway
			w.State.Error = err
			http.Error(w, w.State.Error.Error(), w.State.Status)
			if opts.Cache.Cap > 0 {
				opts.Cache.Delete(&safePath)
			}
			return
		}

		if opts.Cache.Cap > 0 {
			opts.Cache.Add(&safePath, buf[:bytesRead], cachedEntry)
		}

		first = false
	}

	if opts.Cache.Cap > 0 {
		cachedEntry.IsLoaded = true
	}
}
