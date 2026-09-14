package scrape

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/tmdb"
	"context"
	"errors"
	"fmt"
	"strings"
)

// 从tmdb刮削元数据
type TmdbMovieImpl struct {
	TmdbBase
}

func NewTmdbMovieImpl(scrapePath *models.ScrapePath, ctx context.Context) *TmdbMovieImpl {
	return &TmdbMovieImpl{
		TmdbBase: TmdbBase{
			scrapePath: scrapePath,
			ctx:        ctx,
			Client:     models.GlobalScrapeSettings.GetTmdbClient(),
		},
	}
}

// 检查电影是否存在
func (t *TmdbMovieImpl) CheckByNameAndYear(name string, year int, switchYear bool) (string, int64, int, error) {
	language := models.GlobalScrapeSettings.GetTmdbLanguage()
	// 查询电影详情
	movieDetail, err := t.Client.SearchMovie(name, year, language, true, !switchYear)
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电影详情失败, 下次重试, 失败原因: %v", err)
		return "", 0, 0, err
	}
	if movieDetail != nil && movieDetail.TotalResults > 0 {
		// 多条结果时按「名称相似度 + 年份一致」挑最匹配的一条，
		// 避免同名影片一律要求手工输入 tmdb id
		idx := 0
		if movieDetail.TotalResults > 1 {
			bestIdx, bestScore := -1, 0.0
			for i, r := range movieDetail.Results {
				score := NameSimilarity(name, r.Title)
				if s2 := NameSimilarity(name, r.OriginalTitle); s2 > score {
					score = s2
				}
				// 名称完全一致（忽略大小写）直接给满分
				if strings.EqualFold(r.Title, name) || strings.EqualFold(r.OriginalTitle, name) {
					score = 1.0
				}
				// 年份一致加分
				if year > 0 && helpers.ParseYearFromDate(r.ReleaseDate) == year {
					score += YearMatchBonus
				}
				if score > bestScore {
					bestScore = score
					bestIdx = i
				}
			}
			if bestIdx < 0 || bestScore < AutoPickMinScore {
				helpers.AppLogger.Infof("tmdb查询到多部电影，且无法确定唯一匹配，第一个电影标题 %s => %s， 年份 %d", movieDetail.Results[0].Title, name, year)
				errorStr := fmt.Sprintf("通过名称 %s 年份 %d 在TMDB查询到多部电影且无法自动确定，需要手工重新识别输入确定的tmdb id", name, year)
				helpers.AppLogger.Error(errorStr)
				return "", 0, 0, errors.New("多条记录")
			}
			helpers.AppLogger.Infof("tmdb查询到多部电影，自动选择最匹配的：%s => %s（得分 %.2f）",
				name, movieDetail.Results[bestIdx].Title, bestScore)
			idx = bestIdx
		}
		first := movieDetail.Results[idx]
		// TMDB 详情接口在中文语言下仍可能返回英文标题，这里尽量取中文标题
		title := t.Client.GetMovieChineseTitle(first.ID, language, first.Title)
		if title == "" {
			title = first.Title
		}
		return title, first.ID, helpers.ParseYearFromDate(first.ReleaseDate), nil
	} else if movieDetail != nil && movieDetail.TotalResults == 0 {
		if switchYear {
			// 换一个年份字段
			return t.CheckByNameAndYear(name, year, !switchYear)
		}
		return "", 0, 0, errors.New("tmdb没有数据")
	} else {
		return "", 0, 0, errors.New("tmdb没有数据")
	}
}

// 去tmdb查询是否存在
func (t *TmdbMovieImpl) CheckByTmdbId(tmdbId int64) (string, int, error) {
	language := models.GlobalScrapeSettings.GetTmdbLanguage()
	// 查询电影详情
	movieDetail, err := t.Client.GetMovieDetail(tmdbId, language)
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电影详情失败, 下次重试, 失败原因: %v", err)
		return "", 0, err
	}
	// TMDB 详情接口在中文语言下仍可能返回英文标题，这里尽量取中文标题
	title := t.Client.GetMovieChineseTitle(tmdbId, language, movieDetail.Title)
	if title == "" {
		title = movieDetail.Title
	}
	return title, helpers.ParseYearFromDate(movieDetail.ReleaseDate), nil
}

// 检查季是否存在
func (t *TmdbMovieImpl) CheckSeasonByTmdbId(tmdbId int64, seasonNumber int) (*tmdb.SeasonDetail, error) {
	return nil, nil
}
