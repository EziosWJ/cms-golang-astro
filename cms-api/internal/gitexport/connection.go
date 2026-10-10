package gitexport

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"gorm.io/gorm"
)

type Connection struct {
	ID               int64     `gorm:"primaryKey" json:"-"`
	Login            string    `json:"login"`
	Repository       string    `json:"repository"`
	Branch           string    `json:"branch"`
	SourceRepository string    `json:"sourceRepository"`
	SourceRef        string    `json:"sourceRef"`
	TokenCipher      string    `json:"-"`
	HasCredential    bool      `gorm:"-" json:"hasCredential"`
	CredentialError  bool      `gorm:"-" json:"credentialError"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (Connection) TableName() string { return "cms_git_connection" }

type ConnectionInput struct {
	Token            string `json:"token"`
	Repository       string `json:"repository"`
	Branch           string `json:"branch"`
	SourceRepository string `json:"sourceRepository"`
	SourceRef        string `json:"sourceRef"`
}
type Service struct {
	DB     *gorm.DB
	Root   string
	GitHub GitHub
	mu     sync.Mutex
}

func NewService(db *gorm.DB, root string) *Service {
	return &Service{DB: db, Root: filepath.Join(root, "git-export")}
}
func (s *Service) Read(ctx context.Context) (Connection, error) {
	var out Connection
	err := s.DB.WithContext(ctx).Where("id=1").Take(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Connection{SourceRepository: "EziosWJ/cms-golang-astro", SourceRef: "main"}, nil
	}
	if err != nil {
		return out, err
	}
	out.HasCredential = out.TokenCipher != ""
	if out.HasCredential {
		_, err := s.decrypt(out.TokenCipher)
		out.CredentialError = err != nil
	}
	return out, nil
}
func (s *Service) key(create bool) ([]byte, error) {
	path := filepath.Join(s.Root, "master.key")
	raw, err := os.ReadFile(path)
	if err == nil {
		info, statErr := os.Stat(path)
		if statErr != nil || info.Mode().Perm() != 0600 || len(raw) != 32 {
			return nil, ErrCredential
		}
		return raw, nil
	}
	if !os.IsNotExist(err) || !create {
		return nil, ErrCredential
	}
	if err = os.MkdirAll(s.Root, 0700); err != nil {
		return nil, ErrCredential
	}
	if err = os.Chmod(s.Root, 0700); err != nil {
		return nil, ErrCredential
	}
	raw = make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return nil, ErrCredential
	}
	// O_EXCL never overwrites an existing key after a race or recovery.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return s.key(false)
	}
	if err != nil {
		return nil, ErrCredential
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return nil, ErrCredential
	}
	return raw, nil
}
func (s *Service) encrypt(token string, create bool) (string, error) {
	key, err := s.key(create)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrCredential
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrCredential
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", ErrCredential
	}
	return base64.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(token), []byte("cms-github-connection-v1"))), nil
}
func (s *Service) decrypt(value string) (string, error) {
	key, err := s.key(false)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", ErrCredential
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrCredential
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(raw) < aead.NonceSize() {
		return "", ErrCredential
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte("cms-github-connection-v1"))
	if err != nil {
		return "", ErrCredential
	}
	return string(plain), nil
}
func (s *Service) token(ctx context.Context, provided string) (string, error) {
	if provided != "" {
		if len(provided) > 4096 || strings.ContainsAny(provided, "\r\n") {
			return "", ErrCredential
		}
		return provided, nil
	}
	c, err := s.Read(ctx)
	if err != nil {
		return "", err
	}
	return s.decrypt(c.TokenCipher)
}
func validBranch(b string) bool {
	return b != "" && len(b) <= 200 && !strings.HasPrefix(b, "-") && !strings.HasPrefix(b, "/") && !strings.HasSuffix(b, "/") && !strings.HasSuffix(b, ".") && !strings.Contains(b, "..") && !strings.Contains(b, "@{") && !strings.Contains(b, "//") && !strings.ContainsAny(b, " ~^:?*[\\\t\r\n") && b != "@" && !strings.HasSuffix(b, ".lock")
}
func (s *Service) Save(ctx context.Context, in ConnectionInput, meta audit.Metadata) (Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !repoPattern.MatchString(in.Repository) || !repoPattern.MatchString(in.SourceRepository) || !validBranch(in.Branch) || in.SourceRef == "" || len(in.SourceRef) > 200 {
		return Connection{}, ErrInvalid
	}
	old, err := s.Read(ctx)
	if err != nil {
		return old, err
	}
	token, err := s.token(ctx, in.Token)
	if err != nil {
		return old, err
	}
	detection, err := s.GitHub.Detect(ctx, token)
	if err != nil {
		return old, err
	}
	repo, err := s.GitHub.repository(ctx, token, in.Repository)
	if err != nil {
		return old, err
	}
	if !repo.Permissions.Push {
		return old, ErrPushPermission
	}
	branches, err := s.GitHub.Options(ctx, token, in.Repository, "branches")
	if err != nil {
		return old, err
	}
	if len(branches) == 0 && repo.DefaultBranch != in.Branch {
		return old, ErrBranch
	}
	if len(branches) > 0 {
		found := false
		for _, b := range branches {
			found = found || b.Name == in.Branch
		}
		if !found {
			return old, ErrBranch
		}
	}
	if _, err = s.GitHub.Resolve(ctx, in.SourceRepository, in.SourceRef, token); err != nil {
		return old, err
	}
	// An existing ciphertext with a missing key must first be disconnected. Do not replace its key silently.
	ciphertext, err := s.encrypt(token, old.TokenCipher == "")
	if err != nil {
		return old, err
	}
	next := Connection{ID: 1, Login: detection.Login, Repository: in.Repository, Branch: in.Branch, SourceRepository: in.SourceRepository, SourceRef: in.SourceRef, TokenCipher: ciphertext, UpdatedAt: time.Now().UTC()}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if old.ID != 0 {
			if err := s.lockConnection(tx); err != nil {
				return err
			}
			if err := s.active(tx); err != nil {
				return err
			}
		}
		if err := tx.Save(&next).Error; err != nil {
			return err
		}
		return audit.RecordOn(ctx, tx, audit.Event{Action: "UPDATE", Resource: "integration.git.connection", ResourceID: 1, Metadata: meta})
	})
	if err != nil {
		return old, err
	}
	return s.Read(ctx)
}
func (s *Service) Disconnect(ctx context.Context, meta audit.Metadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockConnection(tx); err != nil {
			return err
		}
		if err := s.active(tx); err != nil {
			return err
		}
		if err := tx.Where("id=1").Delete(&Connection{}).Error; err != nil {
			return err
		}
		return audit.RecordOn(ctx, tx, audit.Event{Action: "DELETE", Resource: "integration.git.connection", ResourceID: 1, Metadata: meta})
	})
}
