package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// VerifyExport checks exact private input contents, including metadata identity.
func VerifyExport(directory string, source SourceVersion, hash string) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	raw, err := root.ReadFile("export.json")
	if err != nil {
		return err
	}
	var m ExportMetadata
	if json.Unmarshal(raw, &m) != nil || m.SchemaVersion != 1 || m.Source != source || m.InputHash != hash || len(m.Files) == 0 {
		return errors.New("export metadata mismatch")
	}
	identity := struct {
		SchemaVersion int               `json:"schemaVersion"`
		Source        SourceVersion     `json:"source"`
		Files         map[string]string `json:"files"`
	}{m.SchemaVersion, m.Source, m.Files}
	raw, err = json.Marshal(identity)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != hash {
		return errors.New("export input identity mismatch")
	}
	for path, expected := range m.Files {
		if path == "export.json" {
			return errors.New("metadata cannot checksum itself")
		}
		if _, err = safeJoin(directory, path); err != nil {
			return err
		}
		raw, err = root.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != expected {
			return errors.New("export checksum mismatch")
		}
	}
	return filepath.WalkDir(directory, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("symlink in export")
		}
		if entry.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(directory, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if rel != "export.json" {
			if _, ok := m.Files[rel]; !ok {
				return errors.New("unlisted file in export")
			}
		}
		return nil
	})
}
