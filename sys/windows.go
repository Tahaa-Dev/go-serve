//go:build windows

package sys

func SetRLimit(limit uint64) error {
	return nil
}
