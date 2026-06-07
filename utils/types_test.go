package utils_test

import (
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
		case len(c2.LFUBuckets[i]) != 0 || cap(c2.LFUBuckets[i]) != 0:
			t.Errorf(
				"Expected bucket %d c2 capacity and length to be 0, found:\n Capacity: %d\n Length: %d",
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
