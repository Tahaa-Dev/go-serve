//go:build !windows

package sys

import "syscall"

func SetRLimit(limit uint64) error {
	if limit == 0 {
		return nil
	}

	var rlimit syscall.Rlimit

	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return err
	}

	rlimit.Cur = min(rlimit.Max, limit)

	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit)
}
