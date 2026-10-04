//go:build !windows

package deployment

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func lock(file *os.File) error { return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
func replace(source, target string) error {
	if err := os.Rename(source, target); err != nil {
		return err
	}
	return SyncDirectory(filepath.Dir(target))
}

func SyncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
