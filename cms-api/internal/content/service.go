package content

import (
	"context"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
)

type Store interface {
	Lifecycle(context.Context, int64, string, int64, audit.Metadata) (Detail, error)
	CanEdit(context.Context, int64) (bool, error)
	Page(context.Context, Query) (Page, error)
	Detail(context.Context, int64) (Detail, error)
	Create(context.Context, DraftInput, audit.Metadata) (Detail, error)
	Save(context.Context, int64, SaveInput, audit.Metadata) (Detail, error)
	Revisions(context.Context, int64, Query) (RevisionPage, error)
	Revision(context.Context, int64, int64) (Revision, error)
}
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) authorize(ctx context.Context, actorID int64) error {
	if actorID < 1 {
		return ErrForbidden
	}
	allowed, err := s.store.CanEdit(ctx, actorID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}
func (s *Service) Page(ctx context.Context, actorID int64, q Query) (Page, error) {
	if err := s.authorize(ctx, actorID); err != nil {
		return Page{}, err
	}
	if q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 || q.Page > math.MaxInt/q.PageSize || len(q.Title) > 1000 {
		return Page{}, ErrInvalid
	}
	if q.Lifecycle == "" {
		q.Lifecycle = "active"
	}
	if q.Lifecycle != "active" && q.Lifecycle != "archived" {
		return Page{}, ErrInvalid
	}
	return s.store.Page(ctx, q)
}
func (s *Service) Detail(ctx context.Context, actorID, id int64) (Detail, error) {
	if err := s.authorize(ctx, actorID); err != nil {
		return Detail{}, err
	}
	return s.store.Detail(ctx, id)
}
func (s *Service) Create(ctx context.Context, meta audit.Metadata, in DraftInput) (Detail, error) {
	if err := s.authorize(ctx, meta.ActorID); err != nil {
		return Detail{}, err
	}
	if err := validDraft(in); err != nil {
		return Detail{}, err
	}
	return s.store.Create(ctx, in, meta)
}
func (s *Service) Save(ctx context.Context, meta audit.Metadata, id int64, in SaveInput) (Detail, error) {
	if err := s.authorize(ctx, meta.ActorID); err != nil {
		return Detail{}, err
	}
	if in.ExpectedVersion == nil || *in.ExpectedVersion < 1 || *in.ExpectedVersion == math.MaxInt64 || (in.Mode != "manual" && in.Mode != "autosave") {
		return Detail{}, ErrInvalid
	}
	if err := validDraft(in.DraftInput); err != nil {
		return Detail{}, err
	}
	return s.store.Save(ctx, id, in, meta)
}

func validDraft(in DraftInput) error {
	if !utf8.ValidString(in.Title) || !utf8.ValidString(in.Markdown) || !utf8.ValidString(in.Summary) || utf8.RuneCountInString(in.Title) > 500 || len(in.Markdown) > 2*1024*1024 || utf8.RuneCountInString(in.Summary) > 5000 {
		return ErrInvalid
	}
	if in.DisplayDate != nil && (in.DisplayDate.Year() < 1 || in.DisplayDate.Year() > 9999) {
		return ErrInvalid
	}
	if !utf8.ValidString(in.Slug) || utf8.RuneCountInString(in.Slug) > 200 || in.Slug == "." || in.Slug == ".." || strings.ContainsAny(in.Slug, "/\\%?#") {
		return ErrSlugInvalid
	}
	for _, r := range in.Slug {
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return ErrSlugInvalid
		}
	}
	return nil
}

func (s *Service) Revisions(ctx context.Context, actor, id int64, q Query) (RevisionPage, error) {
	if err := s.authorize(ctx, actor); err != nil {
		return RevisionPage{}, err
	}
	if q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 || q.Page > math.MaxInt/q.PageSize {
		return RevisionPage{}, ErrInvalid
	}
	return s.store.Revisions(ctx, id, q)
}
func (s *Service) Revision(ctx context.Context, actor, id, revisionID int64) (Revision, error) {
	if err := s.authorize(ctx, actor); err != nil {
		return Revision{}, err
	}
	return s.store.Revision(ctx, id, revisionID)
}
func (s *Service) Restore(ctx context.Context, meta audit.Metadata, id, revisionID int64, version *int64) (Detail, error) {
	if err := s.authorize(ctx, meta.ActorID); err != nil {
		return Detail{}, err
	}
	if version == nil || *version < 1 || *version == math.MaxInt64 {
		return Detail{}, ErrInvalid
	}
	rev, err := s.store.Revision(ctx, id, revisionID)
	if err != nil {
		return Detail{}, err
	}
	return s.store.Save(ctx, id, SaveInput{DraftInput: DraftInput{Title: rev.Title, Markdown: rev.Markdown, Summary: rev.Summary, Slug: rev.Slug, DisplayDate: rev.DisplayDate, CoverMediaID: rev.CoverMediaID}, ExpectedVersion: version, Mode: "restore", RestoredTaxonomy: rev.Taxonomy}, meta)
}

func (s *Service) Lifecycle(ctx context.Context, meta audit.Metadata, id int64, lifecycle string, version *int64) (Detail, error) {
	if err := s.authorize(ctx, meta.ActorID); err != nil {
		return Detail{}, err
	}
	if version == nil || *version < 1 || (lifecycle != "active" && lifecycle != "archived") {
		return Detail{}, ErrInvalid
	}
	return s.store.Lifecycle(ctx, id, lifecycle, *version, meta)
}

// ValidateDraft is shared by controlled import preflight and HTTP editing.
func ValidateDraft(in DraftInput) error { return validDraft(in) }
