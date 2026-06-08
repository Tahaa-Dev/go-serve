package utils_test

import (
	"bytes"
	"testing"

	"github.com/Tahaa-Dev/go-serve/utils"
)

func TestNewCache(t *testing.T) {
	c1 := utils.NewCache(0)
	c2 := utils.NewCache(1)
	c3 := utils.NewCache(256)

	for i := range 64 {
		switch {
		case len(c1.LFUBuckets[i]) != 0 || cap(c1.LFUBuckets[i]) != 0:
			t.Errorf(
				"Expected bucket %d in c1 capacity and length to be 0, found:\n Capacity: %d\n Length: %d",
				i,
				cap(c1.LFUBuckets[i]),
				len(c1.LFUBuckets[i]),
			)
		case len(c2.LFUBuckets[i]) != 0 || cap(c2.LFUBuckets[i]) != 1:
			t.Errorf(
				"Expected bucket %d c2 capacity to be 1 and length to be 0, found:\n Capacity: %d\n Length: %d",
				i,
				cap(c2.LFUBuckets[i]),
				len(c2.LFUBuckets[i]),
			)
		case len(c3.LFUBuckets[i]) != 0 || cap(c3.LFUBuckets[i]) != 85:
			t.Errorf(
				"Expected bucket %d c3 capacity to be 85 and length to be 0, found:\n Capacity: %d\n Length: %d",
				i,
				cap(c3.LFUBuckets[i]),
				len(c3.LFUBuckets[i]),
			)
		}
	}
}

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
	if entry.Freq != 1 {
		t.Errorf("Expected entry.Freq to be 1, found: %d", entry.Freq)
	}
	if cache.MinFreq != 1 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq)
	}
}

func TestCacheAddNoEvict(t *testing.T) {
	cache := utils.NewCache(32)
	filename := "/page.html"
	data := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>test</h1>\n</body>\n</html>")
	cache.Add(&filename, data, cache.Get(&filename))

	if cache.Size != 1 {
		t.Errorf("Unexpected cache.Size: %d", cache.Size)
	}
	if cache.MinFreq != 0 {
		t.Errorf("Unexpected cache.MinFreq1: %d", cache.MinFreq)
	}
	if idx := bytes.Index(cache.LFUBuckets[0], []byte(filename)); idx != 0 {
		t.Errorf("Unexpected entry LFUBuckets[0] position: %d", idx)
	}
	if cache.Get(&filename).Freq != 1 {
		t.Errorf("Unexpected entry.Freq: %d", cache.Get(&filename).Freq)
	}
	if idx := bytes.Index(cache.LFUBuckets[1], []byte(filename)); idx != 0 {
		t.Errorf("Unexpected entry LFUBuckets[1] position: %d", idx)
	}
	if cache.MinFreq != 1 {
		t.Errorf("Unexpected cache.MinFreq2: %d", cache.MinFreq)
	}
}

func TestCacheAddEvict(t *testing.T) {
	cache := utils.NewCache(1)
	filename1 := "/page.html"
	data1 := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h1>test</h1>\n</body>\n</html>")
	cache.Add(&filename1, data1, cache.Get(&filename1))

	if cache.MinFreq != 0 {
		t.Errorf("Unexpected cache.MinFreq1: %d", cache.MinFreq)
	}
	if idx := bytes.Index(cache.LFUBuckets[0], []byte(filename1)); idx != 0 {
		t.Errorf("Unexpected entry LFUBuckets[0] position: %d", idx)
	}
	if data := cache.Get(&filename1).Data; !bytes.Equal(data1, data) {
		t.Errorf("Unexpected entry.Data:\n%s", data)
	}

	filename2 := "/page2.html"
	data2 := []byte("<!DOCTYPE html>\n<html>\n<body>\n<h2>test2</h2>\n</body>\n</html>")
	cache.Add(&filename2, data2, cache.Get(&filename2))

	if cache.MinFreq != 1 {
		t.Errorf("Unexpected cache.MinFreq: %d", cache.MinFreq)
	}
	if idx := bytes.Index(cache.LFUBuckets[1], []byte(filename2)); idx != 0 {
		t.Errorf("Unexpected entry LFUBuckets[1] position: %d", idx)
	}
	if data := cache.Get(&filename2).Data; !bytes.Equal(data2, data) {
		t.Errorf("Unexpected entry.Data:\n%s", data)
	}
}
