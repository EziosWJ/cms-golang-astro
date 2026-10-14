// Package siteconfig owns working site/author data and immutable config revisions.
package siteconfig

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	platform "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/http"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
	"gorm.io/gorm"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

// 站点维度常量（ADR-0009 决策一）。
//
// 站点配置从单行单例改为集合后，「本地链路使用的那一份」由 role 决定，不再由 id
// 或站点名决定。local 行的 id 由数据库固定为 1（见 migrations/schema/00015），但
// 代码不得把 1 写成站点身份——一律通过 LocalSite / site_key 解析。
const (
	// LocalSiteKey 是 local 站点的不可变标识，用于解析本地链路使用的那一份配置。
	LocalSiteKey = "local"

	// RoleLocal 与 RoleRemote 是站点配置的角色，写入后由数据库触发器保证不可变。
	RoleLocal  = "local"
	RoleRemote = "remote"

	// StatusEnabled / StatusDisabled 是停用状态（仓库惯例 1=启用 0=停用）。
	// 停用可逆，且「停用」不等于「下线线上站点」——下线是独立的、需二次确认的动作。
	StatusEnabled  = 1
	StatusDisabled = 0
)

// 审计动作。Action 会落进 sys_oper_log.operation_type（P7-04 会断言它），
// 因此按实际动作区分，不再统一硬编码为 "UPDATE"。
const (
	AuditActionCreate  = "CREATE"
	AuditActionUpdate  = "UPDATE"
	AuditActionDisable = "DISABLE"
	AuditActionEnable  = "ENABLE"
)

// remoteDeliveryNote 是给非 local 站点保存/读取时的提示：保存即生效于下一次交付，
// 不产生发布任务，也不与 local 的「发布」混淆。前端文案由 P7-03 负责，但接口必须
// 把这件事说清楚。
const remoteDeliveryNote = "该站点不是本地站点：保存立即生效于下一次向该站点的交付，不会产生发布任务。发布任务与「已上线配置」只服务本地站点。"

var (
	ErrConflict = errors.New("站点配置已被修改，请保留本地修改并重新读取")
	// ErrSiteNotFound 用于按 site_key 寻址但站点不存在的场景。
	ErrSiteNotFound = errors.New("站点不存在")
	// ErrSiteKeyTaken 表示站点标识已被占用（数据库唯一约束为最终保证）。
	ErrSiteKeyTaken = errors.New("站点标识已存在，请更换")
	// ErrSiteKeyInvalid 表示站点标识不符合格式要求。
	ErrSiteKeyInvalid     = errors.New("站点标识须为 1–63 位小写字母、数字或短横线，且以字母或数字开头")
	ErrLocalSiteImmutable = errors.New("本地站点不可停用")
	ErrInvalidStatus      = errors.New("状态值无效，只接受 1（启用）或 0（停用）")
)

// siteKeyPattern 给出站点标识的格式：小写字母/数字开头，其后为小写字母/数字/短横线。
var siteKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func validSiteKey(key string) bool { return siteKeyPattern.MatchString(key) }

type Data struct {
	SiteName      string `json:"siteName"`
	Description   string `json:"description"`
	PublicURL     string `json:"publicUrl"`
	Language      string `json:"language"`
	Timezone      string `json:"timezone"`
	AuthorName    string `json:"authorName"`
	AuthorBio     string `json:"authorBio"`
	AvatarMediaID *int64 `json:"avatarMediaId"`
	Theme         string `json:"theme"`
	ThemeVersion  string `json:"themeVersion"`
}

// Working 是 cms_site_config 的一行，同时用于接口响应。
// SiteKey / Role / Status 为 P7-01 新增的站点列；站点名仍在 Data.SiteName 里
// （有意不新增列，避免第二份真相）。
type Working struct {
	Published *Revision `gorm:"-" json:"published"`
	// Note 仅在非 local 站点上填充，用于表达「保存不产生发布任务」。
	Note       string    `gorm:"-" json:"note,omitempty"`
	ID         int64     `gorm:"primaryKey" json:"-"`
	SiteKey    string    `json:"siteKey"`
	Role       string    `json:"role"`
	Status     int       `json:"status"`
	Version    int64     `json:"version"`
	Data       Data      `gorm:"serializer:json" json:"data"`
	SavedAt    time.Time `json:"savedAt"`
	SavedBy    int64     `json:"savedBy"`
	RevisionID *int64    `gorm:"-" json:"revisionId"`
}

func (Working) TableName() string { return "cms_site_config" }

// Revision 既是配置修订的读取视图，也是产物契约（builder.Manifest.Config）。
// 其字段与 json tag 不得改动：旧 release 的 manifest 要能继续反序列化（P7-02 约束）。
type Revision struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Version   int64     `json:"version"`
	Data      Data      `gorm:"serializer:json" json:"data"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy int64     `json:"createdBy"`
}

func (Revision) TableName() string { return "cms_config_revision" }

// revisionRow 是 cms_config_revision 的站点归属写入/定位视图。
// 有意不扩展 Revision：Revision 是产物契约，加字段会改变其形状（P7-02 约束），
// 而 site_id 只需要在写入与「按站点+版本定位」时使用。
type revisionRow struct {
	ID        int64     `gorm:"primaryKey"`
	SiteID    int64     `gorm:"column:site_id"`
	Version   int64     `json:"version"`
	Data      Data      `gorm:"serializer:json"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy int64     `json:"createdBy"`
}

func (revisionRow) TableName() string { return "cms_config_revision" }

// SiteSummary 是站点集合的列表项。Published 只对 local 有意义：它表示 local 是否
// 已有线上配置；非 local 恒为 false（非 local 不存在「已上线配置」概念）。
type SiteSummary struct {
	SiteKey   string `json:"siteKey"`
	Role      string `json:"role"`
	Status    int    `json:"status"`
	SiteName  string `json:"siteName"`
	PublicURL string `json:"publicUrl"`
	Version   int64  `json:"version"`
	Published bool   `json:"published"`
}

// SiteInput 是新增站点的输入。SiteKey 由作者提供且只填一次，之后不可变。
type SiteInput struct {
	SiteKey string
	Data    Data
}

// AuditEvent 构造站点配置的审计事件。
//
// 抽成纯函数是有意为可测性做的设计：audit.Event.ResourceID 当前不落库
// （sys_oper_log 没有该列），P7-04 需要用单测断言它，而调用本函数即可免去反射。
func AuditEvent(action string, siteID int64, meta audit.Metadata) audit.Event {
	return audit.Event{Action: action, Resource: "content.siteconfig", ResourceID: siteID, Metadata: meta}
}

// Validate 校验站点数据。它保持纯函数、与站点无关：publishing 与 builder 依赖它
// 的纯函数性（发布时对既有修订复用同一套校验），因此不得把 DB 或站点上下文拖进来，
// 也不得收紧逻辑——收紧会让既有已保存的修订在发布时被判 400。
func Validate(in Data) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(in.SiteName) == "" || utf8.RuneCountInString(in.SiteName) > 200 {
		fields["siteName"] = "站点名称须为 1–200 字符"
	}
	if strings.TrimSpace(in.AuthorName) == "" || utf8.RuneCountInString(in.AuthorName) > 200 {
		fields["authorName"] = "作者名称须为 1–200 字符"
	}
	if utf8.RuneCountInString(in.Description) > 5000 {
		fields["description"] = "简介最多 5000 字符"
	}
	if utf8.RuneCountInString(in.AuthorBio) > 5000 {
		fields["authorBio"] = "作者简介最多 5000 字符"
	}
	u, err := url.Parse(in.PublicURL)
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		fields["publicUrl"] = "公开地址须为 HTTP(S) 站点根地址，不含账号、查询或片段"
	}
	if _, err := language.Parse(in.Language); err != nil || in.Language == "" {
		fields["language"] = "语言标记无效"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		fields["timezone"] = "时区无效"
	}
	themeID := strings.TrimSpace(in.Theme)
	if themeID == "" {
		themeID = DefaultThemeID
	}
	theme, ok := LookupTheme(themeID)
	if !ok {
		fields["theme"] = "请选择有效主题"
	} else if in.ThemeVersion != "" && in.ThemeVersion != theme.Version {
		fields["theme"] = "主题版本已变化，请重新保存站点配置"
	}
	return fields
}

// LocalSite 解析 role='local' 的站点行。数据库保证恰有一条（ADR-0009 决策一）。
func LocalSite(ctx context.Context, db *gorm.DB) (Working, error) {
	var work Working
	err := db.WithContext(ctx).Where("role=?", RoleLocal).Take(&work).Error
	return work, err
}

// Read 读取 local 站点的配置。既有编辑器链路恒为 local（内容权威恒为 local）。
func Read(ctx context.Context, db *gorm.DB) (Working, error) {
	return ReadSite(ctx, db, LocalSiteKey)
}

// ReadSite 按站点标识读取该站点的配置。
func ReadSite(ctx context.Context, db *gorm.DB, siteKey string) (Working, error) {
	var work Working
	if err := db.WithContext(ctx).Where("site_key=?", siteKey).Take(&work).Error; err != nil {
		return work, err
	}
	return decorate(ctx, db, work)
}

// ReadSiteID 按站点行 id 读取配置。
func ReadSiteID(ctx context.Context, db *gorm.DB, id int64) (Working, error) {
	var work Working
	if err := db.WithContext(ctx).Where("id=?", id).Take(&work).Error; err != nil {
		return work, err
	}
	return decorate(ctx, db, work)
}

// decorate 为站点行补上派生字段：兼容旧主题、最新修订 id、以及（仅 local 的）
// 「已上线配置」。非 local 站点不参与「已上线配置」推导，Published 恒为 nil。
func decorate(ctx context.Context, db *gorm.DB, work Working) (Working, error) {
	legacyTheme := strings.TrimSpace(work.Data.Theme) == "" || strings.TrimSpace(work.Data.ThemeVersion) == ""
	work.Data = NormalizeLegacyTheme(work.Data)
	var revision revisionRow
	if err := db.WithContext(ctx).Where("site_id=? AND version=?", work.ID, work.Version).Take(&revision).Error; err == nil {
		if !legacyTheme {
			work.RevisionID = &revision.ID
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return work, err
	}
	if work.Role == RoleLocal {
		published, err := publishedRevision(ctx, db)
		if err != nil {
			return work, err
		}
		work.Published = published
		return work, nil
	}
	work.Published = nil
	work.Note = remoteDeliveryNote
	return work, nil
}

// publishedRevision 推导 local 的「已上线配置」。
//
// cms_publish_state 是单行表，只能表达一个站点的线上状态（ADR-0009 决策二/五），
// 因此该 JOIN 只服务 role='local'；非 local 站点不参与此推导。
func publishedRevision(ctx context.Context, db *gorm.DB) (*Revision, error) {
	var release struct{ Manifest string }
	err := db.WithContext(ctx).Table("cms_release r").Select("r.manifest").Joins("JOIN cms_publish_state s ON s.current_release_id=r.id").Take(&release).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Config Revision `json:"config"`
	}
	if err := json.Unmarshal([]byte(release.Manifest), &manifest); err != nil {
		return nil, err
	}
	return &manifest.Config, nil
}

// RevisionIsLocal 报告该配置修订是否属于 local 站点。
//
// publishing 用它阻止把非 local 的修订当成本地线上配置装载（ADR-0009 决策二：
// 内容权威恒为 local）。这是一个显式查询而非依赖调用方拼接 SQL。
func RevisionIsLocal(ctx context.Context, db *gorm.DB, revisionID int64) (bool, error) {
	var count int64
	err := db.WithContext(ctx).Table("cms_config_revision r").
		Joins("JOIN cms_site_config s ON s.id = r.site_id").
		Where("r.id=? AND s.role=?", revisionID, RoleLocal).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 保存 local 站点配置。既有「保存 → 发布」两步链路完全不变。
func Save(ctx context.Context, tx *gorm.DB, version int64, in Data, meta audit.Metadata) (Working, error) {
	return SaveSite(ctx, tx, LocalSiteKey, version, in, meta)
}

// SaveSite 保存指定站点的配置。
//
// 站点寻址后的保存路径对所有站点一致：乐观锁校验 version、更新站点行、追加一条该
// 站点的修订（写 site_id）、登记媒体引用（owner_id 为解析出的站点行 id）、写审计。
// 保存本身不产生发布任务——cms_publish_task 只由发布域写入，因此非 local 站点
// 的保存即生效于下一次交付，不会产生发布任务。
func SaveSite(ctx context.Context, tx *gorm.DB, siteKey string, version int64, in Data, meta audit.Metadata) (Working, error) {
	var saved Working
	resolved, err := ResolveTheme(in)
	if err != nil {
		return saved, err
	}
	in = resolved
	// 有界重试：跨站点并发保存时，两个事务各自读到同一个 MAX(version) 并算出
	// 同一个 next，站点行各自更新成功，随后两条修订插入撞上 cms_config_revision
	// 的全局唯一约束。这里只对「版本号被抢占」重试，其它错误立即返回。
	var lastErr error
	for attempt := 0; attempt < versionRetryAttempts; attempt++ {
		lastErr = tx.WithContext(ctx).Transaction(func(db *gorm.DB) error {
			var site Working
			if err := db.WithContext(ctx).Where("site_key=?", siteKey).Take(&site).Error; err != nil {
				return err
			}
			now := time.Now().UTC()
			next, err := allocateVersion(db)
			if err != nil {
				return err
			}
			result := db.Model(&Working{}).Where("id=? AND version=?", site.ID, version).Updates(map[string]any{"version": next, "saved_at": now, "saved_by": meta.ActorID})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrConflict
			}
			files, err := media.Resolve(db, "", in.AvatarMediaID)
			if err != nil {
				return err
			}
			saved = Working{ID: site.ID, SiteKey: site.SiteKey, Role: site.Role, Status: site.Status, Version: next, Data: in, SavedAt: now, SavedBy: meta.ActorID}
			if err := db.Model(&Working{}).Where("id=?", site.ID).Select("Data").Updates(&saved).Error; err != nil {
				return err
			}
			revision := revisionRow{SiteID: site.ID, Version: next, Data: in, CreatedAt: now, CreatedBy: meta.ActorID}
			if err := db.Create(&revision).Error; err != nil {
				return classifyRevisionInsert(err)
			}
			saved.RevisionID = &revision.ID
			if err := media.SetReferences(db, "config", site.ID, files); err != nil {
				return err
			}
			if err := media.SetReferences(db, "config_revision", revision.ID, files); err != nil {
				return err
			}
			return audit.RecordOn(ctx, db, AuditEvent(AuditActionUpdate, site.ID, meta))
		})
		if lastErr == nil {
			break
		}
		// 只对「版本号被抢占」重试；ErrConflict、ErrSiteKeyTaken、校验类错误一律立即返回。
		if errors.Is(lastErr, errVersionRace) {
			continue
		}
		break
	}
	if lastErr != nil {
		if errors.Is(lastErr, errVersionRace) {
			// 重试耗尽，版本号仍被持续抢占：给出可读、可操作的冲突提示，
			// 而不是把方言相关的数据库错误抛给调用方。
			return saved, ErrConflict
		}
		return saved, lastErr
	}
	return ReadSite(ctx, tx, siteKey)
}

// CreateSite 新增一个非 local 站点（role='remote'）并追加它的第一条配置修订。
//
// 站点标识由作者提供且只填一次（数据库触发器保证此后不可变）；重名或格式非法时
// 返回可读错误。
//
// 本票不提供「删除非 local 站点」的入口。ADR-0009 规定「已被交付记录引用过的站点
// 配置只能停用，不能删除」，而交付记录表要到 P7-06 才存在；此刻放出删除入口，之后
// 必须再加限制，不如不放。local 不可删除由数据库触发器保证（已具备）。
func CreateSite(ctx context.Context, tx *gorm.DB, in SiteInput, meta audit.Metadata) (Working, error) {
	siteKey := strings.TrimSpace(in.SiteKey)
	if !validSiteKey(siteKey) {
		return Working{}, ErrSiteKeyInvalid
	}
	resolved, err := ResolveTheme(in.Data)
	if err != nil {
		return Working{}, err
	}
	in.Data = resolved
	var created Working
	// 与 SaveSite 相同的有界重试：CreateSite 也在同一事务里分配全局 version
	// 并写入修订，跨站点并发新增同样会撞全局唯一约束。
	var lastErr error
	for attempt := 0; attempt < versionRetryAttempts; attempt++ {
		lastErr = tx.WithContext(ctx).Transaction(func(db *gorm.DB) error {
			var existing int64
			if err := db.Model(&Working{}).Where("site_key=?", siteKey).Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				return ErrSiteKeyTaken
			}
			now := time.Now().UTC()
			next, err := allocateVersion(db)
			if err != nil {
				return err
			}
			files, err := media.Resolve(db, "", in.Data.AvatarMediaID)
			if err != nil {
				return err
			}
			site := Working{SiteKey: siteKey, Role: RoleRemote, Status: StatusEnabled, Version: next, Data: in.Data, SavedAt: now, SavedBy: meta.ActorID}
			if err := db.Create(&site).Error; err != nil {
				// 数据库唯一约束是最终保证；并发下这里仍会命中并给出可读错误。
				// 这条是 site_key 冲突（表上针对 remote 行只有它一个唯一键），
				// 与版本号抢占无关，绝不能被当成可重试条件。
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					return ErrSiteKeyTaken
				}
				return err
			}
			revision := revisionRow{SiteID: site.ID, Version: next, Data: in.Data, CreatedAt: now, CreatedBy: meta.ActorID}
			if err := db.Create(&revision).Error; err != nil {
				return classifyRevisionInsert(err)
			}
			if err := media.SetReferences(db, "config", site.ID, files); err != nil {
				return err
			}
			if err := media.SetReferences(db, "config_revision", revision.ID, files); err != nil {
				return err
			}
			if err := audit.RecordOn(ctx, db, AuditEvent(AuditActionCreate, site.ID, meta)); err != nil {
				return err
			}
			created = site
			return nil
		})
		if lastErr == nil {
			break
		}
		// 只对「版本号被抢占」重试；ErrConflict、ErrSiteKeyTaken、校验类错误一律立即返回。
		if errors.Is(lastErr, errVersionRace) {
			continue
		}
		break
	}
	if lastErr != nil {
		if errors.Is(lastErr, errVersionRace) {
			return created, ErrConflict
		}
		return created, lastErr
	}
	return ReadSite(ctx, tx, siteKey)
}

// SetSiteStatus 启用/停用站点配置。停用可逆；停用不等于下线线上站点。
// local 不可停用（数据库 CHECK 也保证）；非 local 可停用。状态无变化时不改写、不写审计。
func SetSiteStatus(ctx context.Context, tx *gorm.DB, siteKey string, status int, meta audit.Metadata) (Working, error) {
	if status != StatusEnabled && status != StatusDisabled {
		return Working{}, ErrInvalidStatus
	}
	err := tx.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		var site Working
		if err := db.WithContext(ctx).Where("site_key=?", siteKey).Take(&site).Error; err != nil {
			return err
		}
		if site.Role == RoleLocal && status != StatusEnabled {
			return ErrLocalSiteImmutable
		}
		if site.Status == status {
			return nil
		}
		if err := db.Model(&Working{}).Where("id=?", site.ID).Update("status", status).Error; err != nil {
			return err
		}
		action := AuditActionEnable
		if status == StatusDisabled {
			action = AuditActionDisable
		}
		return audit.RecordOn(ctx, db, AuditEvent(action, site.ID, meta))
	})
	if err != nil {
		return Working{}, err
	}
	return ReadSite(ctx, tx, siteKey)
}

// ListSites 列出全部站点配置。Published 只对 local 有意义。
func ListSites(ctx context.Context, db *gorm.DB) ([]SiteSummary, error) {
	var sites []Working
	if err := db.WithContext(ctx).Order("id ASC").Find(&sites).Error; err != nil {
		return nil, err
	}
	published, err := publishedRevision(ctx, db)
	if err != nil {
		return nil, err
	}
	out := make([]SiteSummary, 0, len(sites))
	for _, site := range sites {
		out = append(out, SiteSummary{
			SiteKey:   site.SiteKey,
			Role:      site.Role,
			Status:    site.Status,
			SiteName:  site.Data.SiteName,
			PublicURL: site.Data.PublicURL,
			Version:   site.Version,
			Published: site.Role == RoleLocal && published != nil,
		})
	}
	return out, nil
}

// allocateVersion 分配下一个全局唯一的配置版本号。
//
// cms_config_revision.version 全局唯一（P7-01 决策修订：不重建该表、保持唯一约束），
// 而 Read 依据 (site_id, version) 定位站点最新修订，因此站点行的 version 必须与该
// 站点最新修订的 version 相等。两者唯一能同时成立的做法就是从全局最大值分配——
// 这也正是「版本号在站点间不连续」的由来。只有 local 一个站点时，本分配恰为
// version+1，与 P7-02 之前的 local 行为逐值一致。
func allocateVersion(db *gorm.DB) (int64, error) {
	var next int64
	if err := db.Model(&Working{}).Select("COALESCE(MAX(version),0)+1").Scan(&next).Error; err != nil {
		return 0, err
	}
	return next, nil
}

// versionRetryAttempts 是「全局版本号被并发抢占」时的有界重试次数。
//
// 重试是安全的：外层事务是 all-or-nothing，重试会重新读取站点行、重新分配
// version；媒体引用与审计随回滚一并丢弃，不会产生重复副作用。
const versionRetryAttempts = 3

// errVersionRace 是内部哨兵：本次事务因 cms_config_revision 的全局唯一 version
// 被并发独占而失败，可以安全重试。它不会逃逸出 siteconfig 包——调用方只会看到
// ErrConflict（重试耗尽时）。
var errVersionRace = errors.New("config revision version taken by a concurrent save")

// classifyRevisionInsert 归类「写入配置修订」的失败。
//
// 之所以能只凭「这一步 + gorm.ErrDuplicatedKey」就判定为版本号抢占：
//   - 判定范围被收窄到「插入 cms_config_revision」这一条语句——站点行的
//     site_key 冲突（CreateSite 的场景）发生在更早的语句上，不会走到这里；
//   - cms_config_revision 上除自增主键外只有一个唯一键，就是全局唯一的 version
//     （migrations/schema/00011 与 sqlite/schema/00011），没有任何其它业务唯一约束；
//   - GORM 的 TranslateError 已把方言各异的唯一约束冲突（PostgreSQL 23505 /
//     SQLite SQLITE_CONSTRAINT_UNIQUE，扩展码 2067/1555）统一成 gorm.ErrDuplicatedKey，
//     因此无须解析方言相关的错误文案或约束名。
//
// 即便未来该表新增了其它唯一键，最坏结果也只是把一次真实冲突重试有限次数后
// 归约为可读的 ErrConflict——有界、无副作用、不丢数据，不会把原始数据库错误抛给用户。
func classifyRevisionInsert(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errVersionRace
	}
	return err
}

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db} }

func (h *Handler) Register(r gin.IRouter) {
	// 既有编辑器链路：只服务 local（内容权威恒为 local）。
	r.GET("/site-config/editor-context", authz.RequireAny(h.db, "content:article:edit", "content:config:edit"), func(c *gin.Context) {
		work, err := Read(c.Request.Context(), h.db)
		reply(c, map[string]any{"siteName": work.Data.SiteName, "timezone": work.Data.Timezone, "published": work.Published}, err)
	})
	r.GET("/site-config/editor-timezone", authz.RequireAny(h.db, "content:article:edit", "content:config:edit"), func(c *gin.Context) {
		working, err := Read(c.Request.Context(), h.db)
		reply(c, map[string]string{"timezone": working.Data.Timezone}, err)
	})
	// 既有单站点入口：默认指向 local，响应结构与既有契约兼容。
	g := r.Group("/site-config")
	g.Use(authz.Require(h.db, "content:config:edit"))
	g.GET("", h.get)
	g.GET("/themes", h.themes)
	g.PUT("", h.save)
	// 站点集合管理面：读取/保存任意站点用 content:config:edit。
	g.GET("/sites", h.listSites)
	g.GET("/sites/:siteKey", h.getSite)
	g.PUT("/sites/:siteKey", h.saveSite)
	// 新增站点与停用/启用站点使用新权限码 content:site:manage。
	r.POST("/site-config/sites", authz.Require(h.db, "content:site:manage"), h.createSite)
	r.POST("/site-config/sites/:siteKey/status", authz.Require(h.db, "content:site:manage"), h.setStatus)
}

func (h *Handler) get(c *gin.Context) {
	work, err := Read(c.Request.Context(), h.db)
	reply(c, work, err)
}
func (h *Handler) themes(c *gin.Context) { platform.OK(c, Themes()) }
func (h *Handler) save(c *gin.Context)   { h.saveSiteConfig(c, LocalSiteKey) }
func (h *Handler) saveSite(c *gin.Context) {
	h.saveSiteConfig(c, c.Param("siteKey"))
}

// saveSiteConfig 是 local 与指定站点共用的保存流程：请求体形状、校验与错误映射一致。
func (h *Handler) saveSiteConfig(c *gin.Context, siteKey string) {
	var input struct {
		ExpectedVersion *int64 `json:"expectedVersion"`
		Data            Data   `json:"data"`
	}
	if c.ShouldBindJSON(&input) != nil || input.ExpectedVersion == nil || *input.ExpectedVersion < 1 || *input.ExpectedVersion == math.MaxInt64 {
		platform.WriteError(c, 400, 400, "配置参数无效", nil)
		return
	}
	resolved, err := ResolveTheme(input.Data)
	if err != nil {
		platform.WriteError(c, 400, 400, "配置校验失败", map[string]string{"theme": "请选择有效主题"})
		return
	}
	input.Data = resolved
	fields := Validate(input.Data)
	if len(fields) > 0 {
		platform.WriteError(c, 400, 400, "配置校验失败", fields)
		return
	}
	meta := siteMeta(c)
	work, err := SaveSite(c.Request.Context(), h.db, siteKey, *input.ExpectedVersion, input.Data, meta)
	reply(c, work, err)
}
func (h *Handler) getSite(c *gin.Context) {
	work, err := ReadSite(c.Request.Context(), h.db, c.Param("siteKey"))
	reply(c, work, err)
}
func (h *Handler) listSites(c *gin.Context) {
	sites, err := ListSites(c.Request.Context(), h.db)
	reply(c, sites, err)
}
func (h *Handler) createSite(c *gin.Context) {
	var input struct {
		SiteKey string `json:"siteKey"`
		Data    Data   `json:"data"`
	}
	if c.ShouldBindJSON(&input) != nil {
		platform.WriteError(c, 400, 400, "配置参数无效", nil)
		return
	}
	resolved, err := ResolveTheme(input.Data)
	if err != nil {
		platform.WriteError(c, 400, 400, "配置校验失败", map[string]string{"theme": "请选择有效主题"})
		return
	}
	input.Data = resolved
	fields := Validate(input.Data)
	siteKey := strings.TrimSpace(input.SiteKey)
	if !validSiteKey(siteKey) {
		fields["siteKey"] = ErrSiteKeyInvalid.Error()
	}
	if len(fields) > 0 {
		platform.WriteError(c, 400, 400, "配置校验失败", fields)
		return
	}
	work, err := CreateSite(c.Request.Context(), h.db, SiteInput{SiteKey: siteKey, Data: input.Data}, siteMeta(c))
	if errors.Is(err, ErrSiteKeyTaken) {
		platform.WriteError(c, 409, 409, ErrSiteKeyTaken.Error(), map[string]string{"siteKey": ErrSiteKeyTaken.Error()})
		return
	}
	reply(c, work, err)
}
func (h *Handler) setStatus(c *gin.Context) {
	var input struct {
		Status *int `json:"status"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Status == nil {
		platform.WriteError(c, 400, 400, "状态参数无效", nil)
		return
	}
	work, err := SetSiteStatus(c.Request.Context(), h.db, c.Param("siteKey"), *input.Status, siteMeta(c))
	reply(c, work, err)
}

func siteMeta(c *gin.Context) audit.Metadata {
	p, _ := auth.PrincipalFromContext(c.Request.Context())
	m, _ := platform.RequestMetaFromContext(c.Request.Context())
	return audit.Metadata{ActorID: p.UserID, RequestID: m.RequestID, RequestMethod: c.Request.Method, RequestURL: c.Request.URL.RequestURI()}
}

func reply(c *gin.Context, data any, err error) {
	if err == nil {
		platform.OK(c, data)
		return
	}
	if platform.IsTemporaryUnavailable(err) {
		platform.TemporaryUnavailable(c)
		return
	}
	status := 500
	message := "配置操作失败"
	switch {
	case errors.Is(err, ErrConflict):
		status = 409
		message = err.Error()
	case errors.Is(err, ErrLocalSiteImmutable), errors.Is(err, ErrSiteKeyTaken):
		status = 409
		message = err.Error()
	case errors.Is(err, gorm.ErrRecordNotFound):
		status = 404
		message = ErrSiteNotFound.Error()
	case errors.Is(err, media.ErrInvalid), errors.Is(err, ErrThemeInvalid), errors.Is(err, ErrSiteKeyInvalid), errors.Is(err, ErrInvalidStatus):
		status = 400
		message = err.Error()
	}
	platform.WriteError(c, status, status, message, nil)
}
