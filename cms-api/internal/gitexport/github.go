// Package gitexport transports derived public inputs, independently of local publishing.
package gitexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var ErrPublic = errors.New("首版仅支持公开 GitHub 仓库")
var ErrPushPermission = errors.New("目标仓库没有推送权限")
var ErrBranch = errors.New("请选择已有目标分支；空仓库请使用默认分支")
var ErrInvalid = errors.New("Git 连接参数无效")
var ErrCredential = errors.New("GitHub 凭据不可用，请重新配置连接")
var ErrRemote = errors.New("GitHub 操作失败，请检查网络、凭据、Contents 权限与分支保护")
var ErrConflict = errors.New("存在进行中的 Git 推送，暂不能更改连接")
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type GitHub struct {
	HTTP    *http.Client
	BaseURL string
}
type Repository struct {
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	Permissions   struct {
		Push bool `json:"push"`
	} `json:"permissions"`
}
type RefOption struct {
	Name string `json:"name"`
}
type Detection struct {
	Login        string       `json:"login"`
	Repositories []Repository `json:"repositories"`
}
type remoteError struct{ Status int }

func (e remoteError) Error() string {
	switch e.Status {
	case 401:
		return "GitHub Token 已失效，请重新配置"
	case 403:
		return "GitHub 拒绝访问，请检查 Contents 权限或限流"
	case 404:
		return "GitHub 仓库或分支不存在，或 Token 未获授权"
	case 409:
		return "GitHub 仓库尚未初始化或发生提交冲突"
	case 422:
		return "GitHub 拒绝提交，请检查分支名称和保护规则"
	}
	return ErrRemote.Error()
}
func (g GitHub) request(ctx context.Context, token, method, path string, in, out any) error {
	base := g.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return ErrInvalid
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return ErrRemote
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	client := g.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	response, err := client.Do(req)
	if err != nil {
		return ErrRemote
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return remoteError{response.StatusCode}
	}
	if out == nil {
		return nil
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 16*1024*1024)).Decode(out); err != nil {
		return ErrRemote
	}
	return nil
}
func (g GitHub) Detect(ctx context.Context, token string) (Detection, error) {
	out := Detection{Repositories: []Repository{}}
	if strings.TrimSpace(token) == "" || len(token) > 4096 {
		return out, ErrCredential
	}
	if err := g.request(ctx, token, "GET", "/user", nil, &out); err != nil {
		return out, err
	}
	for page := 1; page <= 20; page++ {
		var repos []Repository
		if err := g.request(ctx, token, "GET", fmt.Sprintf("/user/repos?per_page=100&page=%d&sort=full_name", page), nil, &repos); err != nil {
			return out, err
		}
		for _, repo := range repos {
			if !repo.Private && repo.Permissions.Push {
				out.Repositories = append(out.Repositories, repo)
			}
		}
		if len(repos) < 100 {
			return out, nil
		}
	}
	return out, nil
}
func (g GitHub) repository(ctx context.Context, token, repo string) (Repository, error) {
	var out Repository
	if !repoPattern.MatchString(repo) {
		return out, ErrInvalid
	}
	err := g.request(ctx, token, "GET", "/repos/"+repo, nil, &out)
	if err == nil && out.Private {
		return out, ErrPublic
	}
	return out, err
}
func (g GitHub) Options(ctx context.Context, token, repo, kind string) ([]RefOption, error) {
	if _, err := g.repository(ctx, token, repo); err != nil {
		return nil, err
	}
	if kind != "branches" && kind != "tags" {
		return nil, ErrInvalid
	}
	out := []RefOption{}
	for page := 1; page <= 20; page++ {
		var refs []RefOption
		err := g.request(ctx, token, "GET", fmt.Sprintf("/repos/%s/%s?per_page=100&page=%d", repo, kind, page), nil, &refs)
		if err != nil {
			return nil, err
		}
		out = append(out, refs...)
		if len(refs) < 100 {
			break
		}
	}
	return out, nil
}
func (g GitHub) Resolve(ctx context.Context, repo, ref string) (string, error) {
	if _, err := g.repository(ctx, "", repo); err != nil {
		return "", err
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	err := g.request(ctx, "", "GET", "/repos/"+repo+"/commits/"+url.PathEscape(ref), nil, &commit)
	if err == nil && !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(commit.SHA) {
		return "", ErrRemote
	}
	return commit.SHA, err
}
