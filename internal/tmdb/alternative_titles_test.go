package tmdb

import "testing"

func TestHasChinese(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Spider-Man: No Way Home", false},
		{"蜘蛛侠：英雄无归", true},
		{"你的名字。", true},
		{"Breaking Bad", false},
		{"2012", false},
		{"", false},
		{"你的名字 Your Name", true},
	}
	for _, c := range cases {
		if got := HasChinese(c.in); got != c.want {
			t.Errorf("HasChinese(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsChineseLanguage(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"zh-CN", true},
		{"zh-TW", true},
		{"zh-HK", true},
		{"ZH-CN", true},
		{" zh-CN ", true},
		{"en-US", false},
		{"", false},
		{"ja-JP", false},
	}
	for _, c := range cases {
		if got := IsChineseLanguage(c.in); got != c.want {
			t.Errorf("IsChineseLanguage(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNeedChineseTitle(t *testing.T) {
	if !needChineseTitle("zh-CN", "Spider-Man: No Way Home") {
		t.Error("中文语言 + 英文标题 应当需要查询中文标题")
	}
	if needChineseTitle("zh-CN", "蜘蛛侠：英雄无归") {
		t.Error("标题已含中文，不应再查询")
	}
	if needChineseTitle("en-US", "Spider-Man: No Way Home") {
		t.Error("英文语言配置不应查询中文标题")
	}
}

func TestPickChineseFromAlternatives(t *testing.T) {
	titles := []AlternativeTitle{
		{ISO3166_1: "US", Title: "Spider-Man: No Way Home"},
		{ISO3166_1: "TW", Title: "蜘蛛人：無家日"},
		{ISO3166_1: "HK", Title: "蜘蛛俠：不戰無歸"},
		{ISO3166_1: "CN", Title: "蜘蛛侠：英雄无归"},
	}
	if got := pickChineseFromAlternatives(titles); got != "蜘蛛侠：英雄无归" {
		t.Errorf("应优先取 CN，实际: %q", got)
	}

	// 没有 CN 时退到 TW/HK
	titles2 := []AlternativeTitle{
		{ISO3166_1: "US", Title: "Spider-Man"},
		{ISO3166_1: "TW", Title: "蜘蛛人"},
	}
	if got := pickChineseFromAlternatives(titles2); got != "蜘蛛人" {
		t.Errorf("无 CN 时应取 TW，实际: %q", got)
	}

	// 只有英文备选标题时返回空
	titles3 := []AlternativeTitle{
		{ISO3166_1: "US", Title: "Spider-Man"},
		{ISO3166_1: "GB", Title: "Spider-Man (UK)"},
	}
	if got := pickChineseFromAlternatives(titles3); got != "" {
		t.Errorf("无中文标题时应返回空，实际: %q", got)
	}

	// 注意：电视剧接口同样使用 results 字段，这里复用的就是同一个挑选函数
}

func TestPickChineseFromTranslations(t *testing.T) {
	translations := []Translation{
		{ISO639_1: "en-US", Data: TranslationData{Title: "Breaking Bad"}},
		{ISO639_1: "zh-TW", Data: TranslationData{Title: "絕命毒師"}},
		{ISO639_1: "zh-CN", Data: TranslationData{Title: "绝命毒师"}},
	}
	if got := pickChineseFromTranslations(translations); got != "绝命毒师" {
		t.Errorf("应优先取 zh-CN，实际: %q", got)
	}

	// 电视剧的译名在 data.name 字段
	tvTranslations := []Translation{
		{ISO639_1: "en-US", Data: TranslationData{Name: "Breaking Bad"}},
		{ISO639_1: "zh", Data: TranslationData{Name: "绝命毒师"}},
	}
	if got := pickChineseFromTranslations(tvTranslations); got != "绝命毒师" {
		t.Errorf("应读取 data.name，实际: %q", got)
	}

	// 没有中文翻译时返回空
	if got := pickChineseFromTranslations([]Translation{
		{ISO639_1: "en-US", Data: TranslationData{Title: "Breaking Bad"}},
	}); got != "" {
		t.Errorf("无中文翻译时应返回空，实际: %q", got)
	}
}
