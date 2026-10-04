// Package deployment provides a durable whole-release pointer and worker lock.
package deployment

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

type Pointer struct {
	AttemptID    int64  `json:"attemptId"`
	ReleaseKey   string `json:"releaseKey"`
	ManifestHash string `json:"manifestHash"`
}

var keyPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func Valid(p Pointer) bool {
	return p.AttemptID > 0 && keyPattern.MatchString(p.ReleaseKey) && len(p.ManifestHash) == 64
}
func Read(root string) (*Pointer, error) {
	bytes, err := os.ReadFile(filepath.Join(root, "current.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Pointer
	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, err
	}
	if !Valid(p) {
		return nil, errors.New("unrecognized release pointer")
	}
	return &p, nil
}
func Switch(root string, p Pointer) error {
	if !Valid(p) {
		return errors.New("invalid release pointer")
	}
	bytes, err := json.Marshal(p)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(root, ".pointer-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0640); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(bytes); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return replace(name, filepath.Join(root, "current.json"))
}

type Lock struct{ file *os.File }

func Acquire(root string) (*Lock, error) {
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(root, ".worker.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lock(file); err != nil {
		file.Close()
		return nil, err
	}
	return &Lock{file}, nil
}
func (l *Lock) Close() error { return l.file.Close() }
