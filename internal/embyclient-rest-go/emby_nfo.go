package embyclientrestgo

import (
	"Q115-STRM/internal/helpers"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// EnsureNfoMetadataOnly 把指定媒体库的元数据 / 图片读取器设为「仅 Nfo」。
//
// 背景：QMS 刮削生成的 nfo 里写的是中文标题，但 Emby 默认同时启用 TMDb 拉取器，
// 刷新时会用 TMDb 的英文原名把 nfo 里的中文标题覆盖掉。设为仅 Nfo 后 Emby 只认 nfo。
//
// 安全起见，只对「路径中包含 strm」的媒体库生效（即 QMS 生成的 STRM 库），
// 避免误伤存放真实视频文件、需要 TMDb 刮削的常规媒体库（例如 PT 下载的动漫库）。
func (c *Client) EnsureNfoMetadataOnly(libraryId string) error {
	if libraryId == "" {
		return nil
	}
	vfs, err := c.GetLibraryVirtualFolders()
	if err != nil {
		return err
	}
	for _, vf := range vfs {
		if vf.ID != libraryId {
			continue
		}
		isStrmLibrary := false
		for _, loc := range vf.Locations {
			if strings.Contains(strings.ToLower(loc), "strm") {
				isStrmLibrary = true
				break
			}
		}
		if !isStrmLibrary {
			helpers.AppLogger.Infof("媒体库 %s 不是 STRM 库，跳过 Nfo 读取器设置", vf.Name)
			return nil
		}
		payload := map[string]any{
			"Id":             vf.ID,
			"Name":           vf.Name,
			"Locations":      vf.Locations,
			"CollectionType": vf.CollectionType,
			"LibraryOptions": map[string]any{
				"MetadataFetchers":     []string{"Nfo"},
				"MetadataFetcherOrder": []string{"Nfo"},
				"ImageFetchers":        []string{"Nfo"},
				"ImageFetcherOrder":    []string{"Nfo"},
			},
		}
		body, merr := json.Marshal(payload)
		if merr != nil {
			return merr
		}
		apiURL := fmt.Sprintf("%s/emby/Library/VirtualFolders?api_key=%s", c.embyURL, c.apiKey)
		req, rerr := http.NewRequest("POST", apiURL, bytes.NewReader(body))
		if rerr != nil {
			return rerr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, derr := c.httpClient.Do(req)
		if derr != nil {
			return derr
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("设置媒体库 %s 的元数据读取器失败: %s", vf.Name, resp.Status)
		}
		helpers.AppLogger.Infof("已将媒体库 %s 的元数据读取器设为仅 Nfo（避免 TMDb 覆盖中文标题）", vf.Name)
		return nil
	}
	return nil
}
