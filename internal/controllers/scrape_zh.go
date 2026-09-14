package controllers

import (
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/synccron"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

// zhHanRegexp 用于判断名称里是否已经包含中文
var zhHanRegexp = regexp.MustCompile(`[\p{Han}]`)

// BatchReScrapeZh 批量中文名回填
// @Summary 批量中文名回填
// @Description 把已刮削（renamed）但名称仍为英文的记录，用 TMDB 中文译名重新刮削。
//
//	仅处理「语言配置为 zh*」「名称不含汉字」「tmdbid 有效」的记录。
//	默认 dry_run=true 只列出会被处理的记录，不实际改动。
//
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param dry_run body boolean false "试跑，true 只列出不改动（默认 true）"
// @Param scrape_path_id body integer false "限定刮削目录 ID，0 表示全部"
// @Param media_type body string false "媒体类型 movie/tvshow，空表示全部"
// @Param limit body integer false "最多处理条数，默认 500"
// @Success 200 {object} object
// @Router /scrape/batch-re-scrape-zh [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func BatchReScrapeZh(c *gin.Context) {
	type batchReq struct {
		DryRun       bool   `json:"dry_run"`
		ScrapePathId uint   `json:"scrape_path_id"`
		MediaType    string `json:"media_type"`
		Limit        int    `json:"limit"`
	}
	req := batchReq{DryRun: true, Limit: 500}
	// body 为空时使用默认值，不报错
	_ = c.ShouldBindJSON(&req)
	if req.Limit <= 0 {
		req.Limit = 500
	}

	_, list := models.GetScrapeMediaFiles(1, req.Limit, req.MediaType, string(models.ScrapeMediaStatusRenamed), "")

	type item struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		TmdbId    int64  `json:"tmdb_id"`
		MediaType string `json:"media_type"`
	}
	targets := make([]item, 0, len(list))
	for _, sm := range list {
		if sm == nil {
			continue
		}
		if req.ScrapePathId > 0 && sm.ScrapePathId != req.ScrapePathId {
			continue
		}
		if sm.TmdbId <= 0 {
			continue
		}
		// 名称已经是中文的跳过
		if zhHanRegexp.MatchString(sm.Name) {
			continue
		}
		targets = append(targets, item{ID: sm.ID, Name: sm.Name, TmdbId: sm.TmdbId, MediaType: string(sm.MediaType)})
	}

	data := make(map[string]any)
	data["total"] = len(targets)
	data["dry_run"] = req.DryRun
	data["items"] = targets

	if req.DryRun {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    Success,
			Message: "试跑完成，未做任何改动；确认无误后带 dry_run=false 再次调用",
			Data:    data,
		})
		return
	}

	success, failed := 0, 0
	failList := make([]string, 0)
	for _, t := range targets {
		sm := models.GetScrapeMediaFileById(t.ID)
		if sm == nil {
			failed++
			continue
		}
		if err := sm.ReScrape("", 0, sm.TmdbId, sm.SeasonNumber, sm.EpisodeNumber); err != nil {
			failed++
			failList = append(failList, sm.Name)
			continue
		}
		success++
	}
	if success > 0 {
		// 触发一次回滚整理，让文件回到源目录，下次扫描按新名称重新整理
		synccron.StartScrapeRollbackCron()
	}

	data["success"] = success
	data["failed"] = failed
	data["failed_names"] = failList
	c.JSON(http.StatusOK, APIResponse[any]{
		Code:    Success,
		Message: "批量中文名回填已提交，下次扫描会按中文名重新整理",
		Data:    data,
	})
}
