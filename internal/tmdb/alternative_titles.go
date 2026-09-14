package tmdb

import (
	"Q115-STRM/internal/helpers"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// TMDB 的 /movie/{id} 与 /tv/{id} 详情接口在 language=zh-CN 时，title/name 字段仍可能返回英文原始标题，
// 真正的中文标题只存在于 alternative_titles(CN) 或 translations(zh) 中。
// 本文件在刮削时补齐这一层，使 media 名称、目录名与 nfo 标题为中文。
//
// 参考:
//   https://developers.themoviedb.org/3/movies/get-movie-alternative-titles
//   https://developers.themoviedb.org/3/tv/get-tv-alternative-titles
//   https://developers.themoviedb.org/3/movies/get-movie-translations

// AlternativeTitle 备选标题
type AlternativeTitle struct {
	ISO3166_1 string `json:"iso_3166_1"` // 国家/地区代码
	Title     string `json:"title"`      // 该地区使用的标题
	Type      string `json:"type"`       // 标题类型
}

// MovieAlternativeTitlesResponse 电影备选标题（电影接口使用 titles 字段）
type MovieAlternativeTitlesResponse struct {
	ID     int64              `json:"id"`     // 影片ID
	Titles []AlternativeTitle `json:"titles"` // 备选标题列表
}

// TvAlternativeTitlesResponse 电视剧备选标题（电视剧接口使用 results 字段）
type TvAlternativeTitlesResponse struct {
	ID      int64              `json:"id"`      // 电视剧ID
	Results []AlternativeTitle `json:"results"` // 备选标题列表
}

// TranslationData 翻译内容
type TranslationData struct {
	Title    string `json:"title"`    // 电影标题
	Name     string `json:"name"`     // 电视剧名称
	Overview string `json:"overview"` // 简介
	Homepage string `json:"homepage"` // 首页
	Tagline  string `json:"tagline"`  // 标语
}

// Translation 翻译条目
type Translation struct {
	ISO3166_1   string          `json:"iso_3166_1"`   // 国家/地区代码
	ISO639_1    string          `json:"iso_639_1"`    // 语言代码
	Name        string          `json:"name"`         // 语言名称
	EnglishName string          `json:"english_name"` // 语言英文名称
	Data        TranslationData `json:"data"`         // 翻译内容
}

// TranslationsResponse 翻译列表
type TranslationsResponse struct {
	ID           int64         `json:"id"`           // 影片/电视剧ID
	Translations []Translation `json:"translations"` // 翻译列表
}

// hanRegex 用于判断字符串中是否包含中日韩统一表意文字（这里用于识别中文标题）
var hanRegex = regexp.MustCompile(`[\p{Han}]`)

// chineseTitleCache 缓存查询成功的中文标题，避免同一个 tmdb id 反复请求 TMDB。
// key 形如 "movie:634649" / "tv:1396"，value 为中文标题（非空才会写入缓存）。
var chineseTitleCache sync.Map

// HasChinese 判断字符串是否包含中文字符
func HasChinese(s string) bool {
	return hanRegex.MatchString(s)
}

// IsChineseLanguage 判断 TMDB 语言配置是否为中文（zh-CN / zh-TW / zh-HK ...）
func IsChineseLanguage(language string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "zh")
}

// needChineseTitle 判断是否有必要去查询中文标题：
// 仅当语言配置为中文、且当前标题不含中文时才需要替换
func needChineseTitle(language string, currentTitle string) bool {
	if !IsChineseLanguage(language) {
		return false
	}
	return !HasChinese(currentTitle)
}

// pickChineseFromAlternatives 从备选标题中挑选最合适的中文标题
// 优先级：CN > TW/HK/MO/SG > 其他地区的中文标题
func pickChineseFromAlternatives(titles []AlternativeTitle) string {
	var cn, zhRegion, other string
	for _, item := range titles {
		title := strings.TrimSpace(item.Title)
		if title == "" || !HasChinese(title) {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(item.ISO3166_1)) {
		case "CN":
			if cn == "" {
				cn = title
			}
		case "TW", "HK", "MO", "SG":
			if zhRegion == "" {
				zhRegion = title
			}
		default:
			if other == "" {
				other = title
			}
		}
	}
	if cn != "" {
		return cn
	}
	if zhRegion != "" {
		return zhRegion
	}
	return other
}

// pickChineseFromTranslations 从翻译列表中挑选最合适的中文标题
// 优先级：zh-CN/zh-Hans > 其他 zh 变体 > zh-TW/zh-HK
func pickChineseFromTranslations(translations []Translation) string {
	var zhCN, zhTW, zhOther string
	for _, item := range translations {
		iso639 := strings.ToLower(strings.TrimSpace(item.ISO639_1))
		if !strings.HasPrefix(iso639, "zh") {
			continue
		}
		title := strings.TrimSpace(item.Data.Title)
		if title == "" {
			// 电视剧的翻译名称在 name 字段
			title = strings.TrimSpace(item.Data.Name)
		}
		if title == "" || !HasChinese(title) {
			continue
		}
		switch iso639 {
		case "zh-cn", "zh-hans", "zh-hans-cn":
			if zhCN == "" {
				zhCN = title
			}
		case "zh-tw", "zh-hk", "zh-hant", "zh-hant-tw", "zh-hant-hk":
			if zhTW == "" {
				zhTW = title
			}
		default:
			if zhOther == "" {
				zhOther = title
			}
		}
	}
	if zhCN != "" {
		return zhCN
	}
	if zhOther != "" {
		return zhOther
	}
	return zhTW
}

// https://api.themoviedb.org/3/movie/{movie_id}/alternative_titles
// 查询电影备选标题
func (c *Client) GetMovieAlternativeTitles(movieID int64) (*MovieAlternativeTitlesResponse, error) {
	respResult := MovieAlternativeTitlesResponse{}
	req := c.resty.R().SetMethod("GET").SetResult(&respResult)
	resp, err := c.doRequest(fmt.Sprintf("/movie/%d/alternative_titles", movieID), req, MakeRequestConfig(2, 5, 5))
	if err != nil {
		helpers.TMDBLog.Errorf("获取电影备选标题失败:%+v", err)
		return nil, err
	}
	if !resp.IsSuccess() {
		helpers.TMDBLog.Errorf("获取电影备选标题失败:%s", resp.String())
		return nil, fmt.Errorf("获取电影备选标题失败:%s", resp.String())
	}
	return &respResult, nil
}

// https://api.themoviedb.org/3/movie/{movie_id}/translations
// 查询电影翻译
func (c *Client) GetMovieTranslations(movieID int64) (*TranslationsResponse, error) {
	respResult := TranslationsResponse{}
	req := c.resty.R().SetMethod("GET").SetResult(&respResult)
	resp, err := c.doRequest(fmt.Sprintf("/movie/%d/translations", movieID), req, MakeRequestConfig(2, 5, 5))
	if err != nil {
		helpers.TMDBLog.Errorf("获取电影翻译失败:%+v", err)
		return nil, err
	}
	if !resp.IsSuccess() {
		helpers.TMDBLog.Errorf("获取电影翻译失败:%s", resp.String())
		return nil, fmt.Errorf("获取电影翻译失败:%s", resp.String())
	}
	return &respResult, nil
}

// https://api.themoviedb.org/3/tv/{series_id}/alternative_titles
// 查询电视剧备选标题
func (c *Client) GetTvAlternativeTitles(tvID int64) (*TvAlternativeTitlesResponse, error) {
	respResult := TvAlternativeTitlesResponse{}
	req := c.resty.R().SetMethod("GET").SetResult(&respResult)
	resp, err := c.doRequest(fmt.Sprintf("/tv/%d/alternative_titles", tvID), req, MakeRequestConfig(2, 5, 5))
	if err != nil {
		helpers.TMDBLog.Errorf("获取电视剧备选标题失败:%+v", err)
		return nil, err
	}
	if !resp.IsSuccess() {
		helpers.TMDBLog.Errorf("获取电视剧备选标题失败:%s", resp.String())
		return nil, fmt.Errorf("获取电视剧备选标题失败:%s", resp.String())
	}
	return &respResult, nil
}

// https://api.themoviedb.org/3/tv/{series_id}/translations
// 查询电视剧翻译
func (c *Client) GetTvTranslations(tvID int64) (*TranslationsResponse, error) {
	respResult := TranslationsResponse{}
	req := c.resty.R().SetMethod("GET").SetResult(&respResult)
	resp, err := c.doRequest(fmt.Sprintf("/tv/%d/translations", tvID), req, MakeRequestConfig(2, 5, 5))
	if err != nil {
		helpers.TMDBLog.Errorf("获取电视剧翻译失败:%+v", err)
		return nil, err
	}
	if !resp.IsSuccess() {
		helpers.TMDBLog.Errorf("获取电视剧翻译失败:%s", resp.String())
		return nil, fmt.Errorf("获取电视剧翻译失败:%s", resp.String())
	}
	return &respResult, nil
}

// GetMovieChineseTitle 获取电影的中文标题。
// 仅当 language 为中文且 currentTitle 不含中文时才会请求 TMDB；
// 依次尝试 alternative_titles -> translations，取不到时返回空字符串（调用方保持原标题）。
func (c *Client) GetMovieChineseTitle(movieID int64, language string, currentTitle string) string {
	if movieID <= 0 || !needChineseTitle(language, currentTitle) {
		return ""
	}
	cacheKey := fmt.Sprintf("movie:%d", movieID)
	if cached, ok := chineseTitleCache.Load(cacheKey); ok {
		if title, ok := cached.(string); ok && title != "" {
			return title
		}
	}
	var title string
	if resp, err := c.GetMovieAlternativeTitles(movieID); err == nil && resp != nil {
		title = pickChineseFromAlternatives(resp.Titles)
	}
	if title == "" {
		if resp, err := c.GetMovieTranslations(movieID); err == nil && resp != nil {
			title = pickChineseFromTranslations(resp.Translations)
		}
	}
	if title == "" {
		return ""
	}
	chineseTitleCache.Store(cacheKey, title)
	helpers.TMDBLog.Infof("电影 %d 使用中文标题: %s => %s", movieID, currentTitle, title)
	return title
}

// GetTvChineseTitle 获取电视剧的中文标题，逻辑同 GetMovieChineseTitle。
func (c *Client) GetTvChineseTitle(tvID int64, language string, currentTitle string) string {
	if tvID <= 0 || !needChineseTitle(language, currentTitle) {
		return ""
	}
	cacheKey := fmt.Sprintf("tv:%d", tvID)
	if cached, ok := chineseTitleCache.Load(cacheKey); ok {
		if title, ok := cached.(string); ok && title != "" {
			return title
		}
	}
	var title string
	if resp, err := c.GetTvAlternativeTitles(tvID); err == nil && resp != nil {
		title = pickChineseFromAlternatives(resp.Results)
	}
	if title == "" {
		if resp, err := c.GetTvTranslations(tvID); err == nil && resp != nil {
			title = pickChineseFromTranslations(resp.Translations)
		}
	}
	if title == "" {
		return ""
	}
	chineseTitleCache.Store(cacheKey, title)
	helpers.TMDBLog.Infof("电视剧 %d 使用中文标题: %s => %s", tvID, currentTitle, title)
	return title
}
