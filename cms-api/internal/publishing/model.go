// Package publishing fixes targets, persists a serial queue and records deployments.
package publishing

import (
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"time"
)

var ErrConflict = errors.New("发布请求冲突、目标已变更或存在在途操作")
var ErrInvalid = errors.New("发布内容、修订或幂等键无效")
var ErrNoBaseline = errors.New("请先发布站点配置建立线上基线")
var ErrBlocked = errors.New("部署指针无法核实，已停止新发布，请检查 worker 恢复记录")

type State struct {
	ID               int64 `gorm:"primaryKey"`
	CurrentReleaseID *int64
	Version          int64
	BlockedReason    string
}

func (State) TableName() string { return "cms_publish_state" }

type Task struct {
	Title            string    `gorm:"-" json:"title"`
	TargetSnapshot   string    `json:"-"`
	ID               int64     `gorm:"primaryKey" json:"id"`
	Kind             string    `json:"kind"`
	ArticleID        *int64    `json:"articleId"`
	RevisionID       *int64    `json:"revisionId"`
	ConfigRevisionID *int64    `json:"configRevisionId"`
	Status           string    `json:"status"`
	Error            string    `json:"error"`
	CreatedAt        time.Time `json:"createdAt"`
	CreatedBy        int64     `json:"createdBy"`
	UpdatedAt        time.Time `json:"updatedAt"`
	Attempts         []Attempt `gorm:"-" json:"attempts,omitempty"`
}

func (Task) TableName() string { return "cms_publish_task" }

type Attempt struct {
	ID                int64      `gorm:"primaryKey" json:"id"`
	TaskID            int64      `json:"taskId"`
	Status            string     `json:"status"`
	BaselineReleaseID *int64     `json:"baselineReleaseId"`
	ReleaseKey        string     `json:"releaseKey"`
	Manifest          string     `json:"-"`
	ManifestHash      string     `json:"manifestHash"`
	SwitchIntent      int        `json:"switchIntent"`
	Error             string     `json:"error"`
	CreatedAt         time.Time  `json:"createdAt"`
	FinishedAt        *time.Time `json:"finishedAt"`
}

func (Attempt) TableName() string { return "cms_publish_attempt" }

type Release struct {
	ID           int64      `gorm:"primaryKey" json:"id"`
	AttemptID    int64      `json:"attemptId"`
	ReleaseKey   string     `json:"releaseKey"`
	Manifest     string     `json:"-"`
	ManifestHash string     `json:"manifestHash"`
	CreatedAt    time.Time  `json:"createdAt"`
	CleanedAt    *time.Time `json:"cleanedAt"`
}

func (Release) TableName() string { return "cms_release" }

type Request struct {
	Key         string `gorm:"primaryKey"`
	RequestHash string
	TaskID      int64
	CreatedAt   time.Time
}

func (Request) TableName() string { return "cms_publish_request" }

type PublishedArticle struct {
	ArticleID        int64 `gorm:"primaryKey"`
	RevisionID       int64
	FirstPublishedAt time.Time
	UpdatedAt        time.Time
}

func (PublishedArticle) TableName() string { return "cms_published_article" }

type SubmitInput struct {
	Kind             string            `json:"kind"`
	ArticleID        int64             `json:"articleId"`
	ConfigRevisionID int64             `json:"configRevisionId"`
	Save             content.SaveInput `json:"save"`
}
