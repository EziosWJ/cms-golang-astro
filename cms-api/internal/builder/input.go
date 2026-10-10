package builder

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PrepareInput writes fixed content and validates/copies its media without invoking Node.
// Local publishing and private preview use the same preparation boundary.
func PrepareInput(ctx context.Context, manifest Manifest, directory, publicRoot, uploadsRoot string) (string, error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	input, _, err := Encode(manifest)
	if err != nil {
		return "", err
	}
	inputPath := filepath.Join(directory, "manifest.json")
	if err := os.WriteFile(inputPath, []byte(input), 0600); err != nil {
		return "", err
	}
	markdownRoot := filepath.Join(directory, "markdown")
	if err := os.MkdirAll(markdownRoot, 0700); err != nil {
		return "", err
	}
	for _, article := range manifest.Articles {
		if err := os.WriteFile(filepath.Join(markdownRoot, fmt.Sprintf("%d.md", article.Revision.ArticleID)), []byte(article.Revision.Markdown), 0600); err != nil {
			return "", err
		}
	}
	for _, resource := range manifest.Media {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		target, err := safeJoin(publicRoot, strings.TrimPrefix(resource.Path, "/"))
		if err != nil {
			return "", err
		}
		source, err := safeJoin(uploadsRoot, resource.File.StoragePath)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return "", err
		}
		file, err := os.Open(source)
		if err != nil {
			return "", err
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			file.Close()
			return "", errors.New("media source is not a regular file")
		}
		writer, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
		if err != nil {
			file.Close()
			return "", err
		}
		checksum := md5.New()
		size, copyErr := io.Copy(io.MultiWriter(writer, checksum), io.LimitReader(file, 50*1024*1024+1))
		closeErr := writer.Close()
		file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if size != resource.File.FileSize || hex.EncodeToString(checksum.Sum(nil)) != resource.File.FileMD5 {
			return "", errors.New("media source identity changed")
		}
	}
	return inputPath, nil
}
