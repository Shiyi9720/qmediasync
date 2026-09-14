package scrape

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/tmdb"
	"context"
	"errors"
	"fmt"
)

// 从tmdb刮削元数据
type TmdbTvShowImpl struct {
	TmdbBase
}

func NewTmdbTvShowImpl(scrapePath *models.ScrapePath, ctx context.Context) *TmdbTvShowImpl {
	return &TmdbTvShowImpl{
		TmdbBase: TmdbBase{
			scrapePath: scrapePath,
			ctx:        ctx,
			Client:     models.GlobalScrapeSettings.GetTmdbClient(),
		},
	}
}

// 检查电视剧是否存在
func (t *TmdbTvShowImpl) CheckByNameAndYear(name string, year int, switchYear bool) (string, int64, int, error) {
	language := models.GlobalScrapeSettings.GetTmdbLanguage()
	// 查询电视剧详情
	tvShowDetail, err := t.Client.SearchTv(name, year, language, true)
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电视剧详情失败, 下次重试, 失败原因: %v", err)
		return "", 0, 0, err
	}
	if len(tvShowDetail.Results) == 0 {
		// helpers.AppLogger.Errorf("识别结果 %s %d 查询失败, 失败原因: tmdb没有数据", name, year)
		return "", 0, 0, errors.New("tmdb没有数据")
	}
	first := tvShowDetail.Results[0]
	// TMDB 详情接口在中文语言下仍可能返回英文标题，这里尽量取中文标题
	title := t.Client.GetTvChineseTitle(first.ID, language, first.Name)
	if title == "" {
		title = first.Name
	}
	if tvShowDetail.TotalResults > 1 {
		// 如果第一个数据的name等于name则认为是正确的
		if first.Name == name {
			return title, first.ID, helpers.ParseYearFromDate(first.FirstAirDate), nil
		}
		errorStr := fmt.Sprintf("通过名称 %s 年份 %d 在TMDB无法查询到多部电视剧，需要手工重新识别输入确定的tmdb id", name, year)
		helpers.AppLogger.Error(errorStr)
		return "", 0, 0, errors.New("多条记录")
	} else {
		return title, first.ID, helpers.ParseYearFromDate(first.FirstAirDate), nil
	}
}

// 去tmdb查询是否存在
func (t *TmdbTvShowImpl) CheckByTmdbId(tmdbId int64) (string, int, error) {
	language := models.GlobalScrapeSettings.GetTmdbLanguage()
	// 查询详情
	tvDetail, err := t.Client.GetTvDetail(tmdbId, language)
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电视剧详情失败, 下次重试, 失败原因: %v", err)
		return "", 0, err
	}
	// TMDB 详情接口在中文语言下仍可能返回英文标题，这里尽量取中文标题
	title := t.Client.GetTvChineseTitle(tmdbId, language, tvDetail.Name)
	if title == "" {
		title = tvDetail.Name
	}
	return title, helpers.ParseYearFromDate(tvDetail.FirstAirDate), nil
}

// 检查季是否存在
func (t *TmdbTvShowImpl) CheckSeasonByTmdbId(tmdbId int64, seasonNumber int) (*tmdb.SeasonDetail, error) {
	// 查询季详情
	seasonDetail, err := t.Client.GetTvSeasonDetail(tmdbId, seasonNumber, models.GlobalScrapeSettings.GetTmdbLanguage())
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电视剧季详情失败, 下次重试, 失败原因: %v", err)
		return nil, err
	}
	return seasonDetail, nil
}
