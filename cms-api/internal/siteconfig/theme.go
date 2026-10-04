package siteconfig

import (
	"errors"
	"strings"
)

const (
	DefaultThemeID  = "comic"
	CMSThemeAPIV1   = "1"
)

var ErrThemeInvalid = errors.New("主题不存在或不可用")

type ThemeCompatibility struct {
	CMSThemeAPI string `json:"cmsThemeApi"`
}

type ThemeDefinition struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Version       string             `json:"version"`
	Description   string             `json:"description"`
	Preview       string             `json:"preview"`
	Compatibility ThemeCompatibility `json:"compatibility"`
	Capabilities  []string           `json:"capabilities"`
}

var builtinThemes = []ThemeDefinition{
	{
		ID:          "comic",
		Name:        "Comic",
		Version:     "1.0.0",
		Description: "Neo-Brutalist 漫画风主题，强调粗线条、纸张感和高对比信息卡片。",
		Preview:     "builtin:comic",
		Compatibility: ThemeCompatibility{CMSThemeAPI: CMSThemeAPIV1},
		Capabilities: []string{"home", "article", "archive", "category", "tag", "404"},
	},
	{
		ID:          "vaporwave",
		Name:        "Vaporwave",
		Version:     "1.0.0",
		Description: "深色霓虹 Vaporwave 主题，强调网格、渐变和发光边界。",
		Preview:     "builtin:vaporwave",
		Compatibility: ThemeCompatibility{CMSThemeAPI: CMSThemeAPIV1},
		Capabilities: []string{"home", "article", "archive", "category", "tag", "404"},
	},
}

func Themes() []ThemeDefinition {
	out := make([]ThemeDefinition, len(builtinThemes))
	copy(out, builtinThemes)
	for i := range out {
		out[i].Capabilities = append([]string(nil), out[i].Capabilities...)
	}
	return out
}

func LookupTheme(id string) (ThemeDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, theme := range builtinThemes {
		if theme.ID == id {
			return theme, true
		}
	}
	return ThemeDefinition{}, false
}

func NormalizeLegacyTheme(in Data) Data {
	id := strings.TrimSpace(in.Theme)
	if id == "" {
		id = DefaultThemeID
	}
	if theme, ok := LookupTheme(id); ok {
		in.Theme = theme.ID
		if strings.TrimSpace(in.ThemeVersion) == "" {
			in.ThemeVersion = theme.Version
		}
	}
	return in
}

func ResolveTheme(in Data) (Data, error) {
	theme, ok := LookupTheme(in.Theme)
	if !ok {
		return in, ErrThemeInvalid
	}
	in.Theme = theme.ID
	in.ThemeVersion = theme.Version
	return in, nil
}
