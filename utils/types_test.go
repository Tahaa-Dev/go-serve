package utils_test

import (
	"bytes"
	"testing"

	"github.com/Tahaa-Dev/go-serve/utils"
)

func TestCacheGet(t *testing.T) {
	cache := utils.NewCache(32)
	filename := "/page.html"
	entry := cache.Get(&filename)
	entry.ContentType = "text/html"
	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>test</h1>\n</body>\n</html>")

	cache.Add(
		&filename,
		data,
		entry,
	)

	entry = cache.Get(&filename)

	if entry.ContentType != "text/html" {
		t.Error("Expected entry.ContentType to be 'text/html', found:", entry.ContentType)
	}
	if !bytes.Equal(entry.Data, data) {
		t.Errorf("Unexpected entry.Data:\n%s", entry.Data)
	}
	if entry.Freq.Load() != 1 {
		t.Errorf("Expected entry.Freq to be 1, found: %d", entry.Freq.Load())
	}
	if cache.MinFreq.Load() != 1 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq.Load())
	}
}

func TestCacheAddNoEvict(t *testing.T) {
	cache := utils.NewCache(32)
	filename := "/page.html"
	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>test</h1>\n</body>\n</html>")
	cache.Add(&filename, data, cache.Get(&filename))

	if cache.Size.Load() != 1 {
		t.Errorf("Unexpected cache.Size: %d", cache.Size.Load())
	}
	if cache.MinFreq.Load() != 0 {
		t.Errorf("Unexpected cache.MinFreq1: %d", cache.MinFreq.Load())
	}
	if cache.Get(&filename).Freq.Load() != 1 {
		t.Errorf("Unexpected entry.Freq: %d", cache.Get(&filename).Freq.Load())
	}
	if cache.MinFreq.Load() != 1 {
		t.Errorf("Unexpected cache.MinFreq2: %d", cache.MinFreq.Load())
	}
}

func TestCacheAddEvict(t *testing.T) {
	cache := utils.NewCache(1)
	filename1 := "/page.html"
	data1 := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>test</h1>\n</body>\n</html>")
	cache.Add(&filename1, data1, cache.Get(&filename1))

	if cache.MinFreq.Load() != 0 {
		t.Errorf("Unexpected cache.MinFreq1: %d", cache.MinFreq.Load())
	}
	if data := cache.Get(&filename1).Data; !bytes.Equal(data1, data) {
		t.Errorf("Unexpected entry.Data:\n%s", data)
	}

	filename2 := "/page2.html"
	data2 := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h2>test2</h2>\n</body>\n</html>")
	cache.Add(&filename2, data2, cache.Get(&filename2))

	if cache.MinFreq.Load() != 1 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq.Load())
	}
	if data := cache.Get(&filename2).Data; !bytes.Equal(data2, data) {
		t.Errorf("Unexpected entry.Data:\n%s", data)
	}
}

func TestCacheUpdateExists(t *testing.T) {
	cache := utils.NewCache(4)
	filename := "page.html"
	entry := cache.Get(&filename)

	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>test</h1>\n</body>\n</html>")
	cache.Update(&filename, data, entry, true)

	if !bytes.Equal(data, entry.Data) {
		t.Errorf("Unexpected data: %s", entry.Data)
	}
	if entry.Freq.Load() != 0 {
		t.Errorf("Unexpected entry.Freq: %d", entry.Freq.Load())
	}
	if cache.Size.Load() != 1 {
		t.Errorf("Unexpected cache.Size: %d", cache.Size.Load())
	}
	if cache.MinFreq.Load() != 0 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq.Load())
	}
}

func TestCacheUpdateNotExists(t *testing.T) {
	cache := utils.NewCache(4)
	filename := "page.html"
	entry := cache.Get(&filename)
	cache.Add(&filename, []byte("wrong data"), entry)

	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>test</h1>\n</body>\n</html>")
	cache.Update(&filename, data, entry, true)

	if !bytes.Equal(data, entry.Data) {
		t.Errorf("Unexpected data: %s", entry.Data)
	}
	if entry.Freq.Load() != 0 {
		t.Errorf("Unexpected entry.Freq: %d", entry.Freq.Load())
	}
	if cache.Size.Load() != 1 {
		t.Errorf("Unexpected cache.Size: %d", cache.Size.Load())
	}
	if cache.MinFreq.Load() != 0 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq.Load())
	}
}

func TestCacheDelete(t *testing.T) {
	cache := utils.NewCache(4)
	filename1 := "page1.html"
	filename2 := "page2.html"
	cache.Add(&filename1, []byte("1"), cache.Get(&filename1))
	cache.Add(&filename2, []byte("2"), cache.Get(&filename2))

	cache.Delete(&filename1)

	if cache.Get(&filename1).Data != nil {
		t.Error("1st entry was not deleted")
	}
	if cache.MinFreq.Load() != 0 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq.Load())
	}
}
