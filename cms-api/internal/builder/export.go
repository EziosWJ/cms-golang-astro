package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
)

// PublicSnapshot is the versioned Astro contract, deliberately independent of DB models.
type PublicSnapshot struct {
	Config   PublicConfig    `json:"config"`
	Articles []PublicArticle `json:"articles"`
	Media    []PublicMedia   `json:"media"`
}
type PublicConfig struct {
	Data siteconfig.Data `json:"data"`
}
type PublicArticle struct {
	Revision         PublicRevision `json:"revision"`
	FirstPublishedAt time.Time      `json:"firstPublishedAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}
type PublicRevision struct {
	ArticleID    int64               `json:"articleId"`
	Slug         string              `json:"slug"`
	Title        string              `json:"title"`
	Markdown     string              `json:"markdown"`
	Summary      string              `json:"summary"`
	DisplayDate  *time.Time          `json:"displayDate"`
	CoverMediaID *int64              `json:"coverMediaId"`
	Taxonomy     []taxonomy.Snapshot `json:"taxonomy"`
}
type PublicMedia struct {
	File PublicFile `json:"file"`
	Path string     `json:"path"`
}
type PublicFile struct {
	ID           int64  `json:"id"`
	OriginalName string `json:"originalName"`
	MimeType     string `json:"mimeType"`
}
type SourceVersion struct {
	Repository string `json:"repository"`
	SHA        string `json:"sha"`
}
type ExportMetadata struct {
	SchemaVersion int               `json:"schemaVersion"`
	Source        SourceVersion     `json:"source"`
	InputHash     string            `json:"inputHash"`
	Files         map[string]string `json:"files"`
}

func publicSnapshot(manifest Manifest) (PublicSnapshot, error) {
	out := PublicSnapshot{Config: PublicConfig{manifest.Config.Data}, Articles: []PublicArticle{}, Media: []PublicMedia{}}
	seen := map[int64]bool{}
	for _, a := range manifest.Articles {
		r := a.Revision
		if r.ArticleID <= 0 || seen[r.ArticleID] {
			return out, errors.New("invalid or duplicate published article")
		}
		seen[r.ArticleID] = true
		terms := append([]taxonomy.Snapshot{}, r.Taxonomy...)
		sort.Slice(terms, func(i, j int) bool {
			if terms[i].Kind == terms[j].Kind {
				return terms[i].ID < terms[j].ID
			}
			return terms[i].Kind < terms[j].Kind
		})
		out.Articles = append(out.Articles, PublicArticle{PublicRevision{r.ArticleID, r.Slug, r.Title, r.Markdown, r.Summary, r.DisplayDate, r.CoverMediaID, terms}, a.FirstPublishedAt, a.UpdatedAt})
	}
	sort.Slice(out.Articles, func(i, j int) bool { return out.Articles[i].Revision.ArticleID < out.Articles[j].Revision.ArticleID })
	paths := map[string]bool{}
	for _, m := range manifest.Media {
		if m.File.ID <= 0 || !strings.HasPrefix(m.Path, "/") || paths[m.Path] {
			return out, errors.New("invalid or duplicate published media")
		}
		paths[m.Path] = true
		out.Media = append(out.Media, PublicMedia{PublicFile{m.File.ID, m.File.OriginalName, m.File.MimeType}, m.Path})
	}
	sort.Slice(out.Media, func(i, j int) bool { return out.Media[i].Path < out.Media[j].Path })
	return out, nil
}

// ExportInput prepares private portable inputs from a verified successful release.
// It never reads current drafts, the upload store, or the latest site config.
// destination must be new; callers publish it only after successful completion.
func ExportInput(ctx context.Context, manifest Manifest, manifestHash, releaseRoot, destination string, source SourceVersion) (metadata ExportMetadata, err error) {
	_, actual, err := Encode(manifest)
	if err != nil {
		return metadata, err
	}
	if actual != manifestHash || manifest.PreviewTaskID != 0 {
		return metadata, errors.New("published snapshot identity mismatch")
	}
	if err = Validate(releaseRoot, Marker{AttemptID: manifest.AttemptID, ReleaseKey: manifest.ReleaseKey, ManifestHash: manifestHash}); err != nil {
		return metadata, err
	}
	snapshot, err := publicSnapshot(manifest)
	if err != nil {
		return metadata, err
	}
	root, err := os.OpenRoot(releaseRoot)
	if err != nil {
		return metadata, err
	}
	defer root.Close()
	raw, err := root.ReadFile(".release.json")
	if err != nil {
		return metadata, err
	}
	var marker Marker
	if err = json.Unmarshal(raw, &marker); err != nil {
		return metadata, err
	}
	if err = os.Mkdir(destination, 0700); err != nil {
		return metadata, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(destination)
		}
	}()
	metadata = ExportMetadata{SchemaVersion: 1, Source: source, Files: map[string]string{}}
	write := func(relative string, data []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		target, err := safeJoin(destination, relative)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err = os.WriteFile(target, data, 0600); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		metadata.Files[relative] = hex.EncodeToString(sum[:])
		return nil
	}
	raw, err = json.Marshal(snapshot)
	if err != nil {
		return metadata, err
	}
	if err = write("manifest.json", raw); err != nil {
		return metadata, err
	}
	for _, a := range snapshot.Articles {
		if err = write(fmt.Sprintf("markdown/%d.md", a.Revision.ArticleID), []byte(a.Revision.Markdown)); err != nil {
			return metadata, err
		}
	}
	for _, m := range snapshot.Media {
		relative := strings.TrimPrefix(m.Path, "/")
		if _, err = safeJoin(releaseRoot, relative); err != nil {
			return metadata, err
		}
		expected, ok := marker.Files[relative]
		if !ok {
			return metadata, errors.New("published media missing from release marker")
		}
		var data []byte
		data, err = root.ReadFile(relative)
		if err != nil {
			return metadata, err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != expected {
			return metadata, errors.New("published media checksum mismatch")
		}
		if err = write("public/"+relative, data); err != nil {
			return metadata, err
		}
	}
	// Only deterministic public file identities and pinned source identify an input.
	identity := struct {
		SchemaVersion int               `json:"schemaVersion"`
		Source        SourceVersion     `json:"source"`
		Files         map[string]string `json:"files"`
	}{metadata.SchemaVersion, metadata.Source, metadata.Files}
	raw, err = json.Marshal(identity)
	if err != nil {
		return metadata, err
	}
	sum := sha256.Sum256(raw)
	metadata.InputHash = hex.EncodeToString(sum[:])
	raw, err = json.Marshal(metadata)
	if err != nil {
		return metadata, err
	}
	err = os.WriteFile(filepath.Join(destination, "export.json"), raw, 0600)
	return metadata, err
}
