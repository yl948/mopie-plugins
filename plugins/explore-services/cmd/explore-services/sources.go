package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0.0.0 Safari/537.36"

func (r *runtime) fetch(req listRequest) (listResult, error) {
	switch req.Source {
	case "bilibili":
		return r.fetchBilibili(req)
	case "tencent":
		return r.fetchTencent(req)
	case "cctv":
		return r.fetchCCTV(req)
	case "mango":
		return r.fetchMango(req)
	case "migu":
		return r.fetchMigu(req)
	case "bangumi":
		return r.fetchBangumi(req)
	default:
		return listResult{}, fmt.Errorf("SOURCE_DISABLED: 未支持的来源: %s", req.Source)
	}
}

func requestJSON(ctx context.Context, method, endpoint string, body io.Reader, headers map[string]string, target interface{}) error {
	return requestJSONWithProxy(ctx, method, endpoint, body, headers, "", target)
}

func requestJSONWithProxy(ctx context.Context, method, endpoint string, body io.Reader, headers map[string]string, proxy string, target interface{}) error {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", browserUA)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
			return fmt.Errorf("BANGUMI_PROXY_INVALID: 代理地址无效")
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("UPSTREAM_TIMEOUT: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("UPSTREAM_UNAVAILABLE: HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(target); err != nil {
		return fmt.Errorf("UPSTREAM_SCHEMA_CHANGED: %w", err)
	}
	return nil
}

func (r *runtime) fetchBilibili(req listRequest) (listResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	values := url.Values{
		"type":        {"1"},
		"st":          {bilibiliCategory(req.Category)},
		"season_type": {bilibiliCategory(req.Category)},
		"page":        {strconv.Itoa(req.Page)},
		"pagesize":    {strconv.Itoa(req.PageSize)},
	}
	if req.Sort == "score_desc" {
		values.Set("order", "4")
	}
	for key, value := range req.Filters {
		if value != "" && allowedFilters[key] {
			values.Set(key, value)
		}
	}
	var body bilibiliResponse
	endpoint := "https://api.bilibili.com/pgc/season/index/result?" + values.Encode()
	if err := requestJSON(ctx, http.MethodGet, endpoint, nil, map[string]string{"Referer": "https://www.bilibili.com"}, &body); err != nil {
		return listResult{}, err
	}
	if body.Code != 0 {
		return listResult{}, fmt.Errorf("UPSTREAM_%d: %s", body.Code, body.Message)
	}
	items := make([]exploreItem, 0, len(body.Data.List))
	for _, item := range body.Data.List {
		items = append(items, normalizeBilibili(item, req.Category))
	}
	next := parseBool(body.Data.HasNext)
	return listResult{Items: items, Pagination: pagination{Mode: "page", Page: req.Page, PageSize: req.PageSize, HasNext: &next}}, nil
}

func bilibiliCategory(category string) string {
	values := map[string]string{"tv": "5", "movie": "2", "documentary": "3", "bangumi": "1", "guo": "4", "variety": "7"}
	return values[category]
}

func parseBool(raw json.RawMessage) bool {
	var value bool
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var number int
	if json.Unmarshal(raw, &number) == nil {
		return number != 0
	}
	return false
}

func normalizeBilibili(item bilibiliItem, category string) exploreItem {
	var score *float64
	if value, err := strconv.ParseFloat(item.Score, 64); err == nil && value > 0 {
		score = &value
	}
	mediaType := sourceCategory(category, "bilibili").MediaType
	if category == "movie" || (category == "bangumi" && item.SeasonType == 2) {
		mediaType = "movie"
	}
	status := "ongoing"
	if item.IsFinish == 1 {
		status = "finished"
	}
	var max *float64
	if score != nil {
		value := 10.0
		max = &value
	}
	var scoreSource string
	if score != nil {
		scoreSource = "bilibili"
	}
	var badges []string
	if item.Badge != "" {
		badges = []string{item.Badge}
	}
	return exploreItem{
		Source: "bilibili", SourceItemID: strconv.FormatInt(item.MediaID, 10), SourceSeasonID: strconv.FormatInt(item.SeasonID, 10),
		Title: item.Title, MediaType: mediaType, Category: category, PosterURL: item.Cover, SourceURL: item.Link,
		Score: score, ScoreScaleMax: max, ScoreSource: scoreSource, Status: status, Badges: badges,
		Subtitle: item.Subtitle, EpisodeText: item.IndexShow,
	}
}

func (r *runtime) fetchTencent(req listRequest) (listResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	channel := map[string]string{"tv": "100113", "movie": "100173", "variety": "100109", "documentary": "100105", "bangumi": "100119", "children": "100150"}[req.Category]
	if channel == "" {
		channel = "100113"
	}
	pageParams := map[string]string{"channel_id": channel, "page_type": "channel_operation", "page_id": "channel_list_second_page"}
	if filters := tencentFilters(req.Filters); filters != "" {
		pageParams["filter_params"] = filters
	}
	payload := map[string]interface{}{"page_params": pageParams}
	if req.Page > 1 {
		payload["page_context"] = map[string]string{"data_src_647bd63b21ef4b64b50fe65201d89c6e_page": strconv.Itoa(req.Page - 1)}
	}
	encoded, _ := json.Marshal(payload)
	values := url.Values{"video_appid": {"1000005"}, "vplatform": {"2"}, "vversion_name": {"8.9.10"}, "new_mark_label_enabled": {"1"}}
	endpoint := "https://pbaccess.video.qq.com/trpc.universal_backend_service.page_server_rpc.PageServer/GetPageData?" + values.Encode()
	var body tencentResponse
	if err := requestJSON(ctx, http.MethodPost, endpoint, strings.NewReader(string(encoded)), map[string]string{"Referer": "https://v.qq.com/", "Content-Type": "application/json"}, &body); err != nil {
		return listResult{}, err
	}
	if len(body.Data.ModuleList) < 2 || len(body.Data.ModuleList[1].ModuleData) == 0 {
		return listResult{}, fmt.Errorf("UPSTREAM_SCHEMA_CHANGED: 腾讯视频列表结构为空")
	}
	module := body.Data.ModuleList[1].ModuleData[0]
	items := make([]exploreItem, 0, len(module.Items.Items))
	for _, raw := range module.Items.Items {
		if raw.ItemType != "2" || raw.Params.CID == "" || raw.Params.Title == "" {
			continue
		}
		item := raw.Params
		poster := item.PicV
		if poster == "" {
			poster = item.PicH
		}
		mediaType := sourceCategory(req.Category, "tencent").MediaType
		if req.Category == "movie" {
			mediaType = "movie"
		}
		items = append(items, exploreItem{
			Source: "tencent", SourceItemID: item.CID, Title: item.Title, Year: item.Year, MediaType: mediaType,
			Category: req.Category, PosterURL: poster, SourceURL: "https://v.qq.com/x/cover/" + url.PathEscape(item.CID) + ".html",
			Status: tencentStatus(item.TimeLong), Subtitle: item.Subtitle,
		})
	}
	hasNext := body.Data.HasNextPage || module.ModuleParams.HasNextPage == "true"
	return listResult{Items: items, Pagination: pagination{Mode: "page", Page: req.Page, PageSize: req.PageSize, HasNext: &hasNext}}, nil
}

func tencentFilters(filters map[string]string) string {
	allowed := map[string]bool{"sort": true, "itype": true, "iarea": true, "iyear": true, "language": true, "pay": true, "recommend": true}
	values := url.Values{}
	for key, value := range filters {
		if allowed[key] && value != "" {
			values.Set(key, value)
		}
	}
	return values.Encode()
}

func tencentStatus(value string) string {
	if strings.Contains(value, "全") {
		return "finished"
	}
	return "ongoing"
}

func (r *runtime) fetchCCTV(req listRequest) (listResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	fc := map[string]string{"tv": "电视剧", "movie": "电影", "animation": "动画片", "documentary": "纪录片", "special": "特别节目"}[req.Category]
	if fc == "" {
		fc = "电视剧"
	}
	values := url.Values{"p": {strconv.Itoa(req.Page)}, "n": {strconv.Itoa(req.PageSize)}, "serviceId": {"cbox"}, "sort": {"desc"}, "fc": {fc}}
	for key, value := range req.Filters {
		if (key == "area" || key == "sc" || key == "year" || key == "fl" || key == "channel") && value != "" {
			values.Set(key, value)
		}
	}
	var body cctvResponse
	endpoint := "https://api.cntv.cn/newVideoset/getCboxVideoAlbumList?" + values.Encode()
	if err := requestJSON(ctx, http.MethodGet, endpoint, nil, map[string]string{"Referer": "https://app.cctv.com/"}, &body); err != nil {
		return listResult{}, err
	}
	items := make([]exploreItem, 0, len(body.Data.List))
	for _, item := range body.Data.List {
		if item.ID == "" || item.Title == "" {
			continue
		}
		mediaType := sourceCategory(req.Category, "cctv").MediaType
		if req.Category == "movie" {
			mediaType = "movie"
		}
		items = append(items, exploreItem{Source: "cctv", SourceItemID: item.ID, Title: strings.Trim(item.Title, "《》"), Year: item.Year, MediaType: mediaType, Category: req.Category, PosterURL: item.Image, SourceURL: "https://tv.cctv.com/", Status: "finished", Subtitle: item.Area})
	}
	hasNext := len(items) == req.PageSize
	return listResult{Items: items, Pagination: pagination{Mode: "page", Page: req.Page, PageSize: req.PageSize, Total: body.Data.Total, HasNext: &hasNext}}, nil
}

func (r *runtime) fetchMango(req listRequest) (listResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	channel := map[string]string{"tv": "2", "movie": "3", "bangumi": "50", "children": "10", "variety": "1", "documentary": "51", "education": "115"}[req.Category]
	if channel == "" {
		channel = "2"
	}
	values := url.Values{"allowedRC": {"1"}, "platform": {"pcweb"}, "channelId": {channel}, "pn": {strconv.Itoa(req.Page)}, "pc": {strconv.Itoa(req.PageSize)}, "hudong": {"1"}, "_support": {"10000000"}}
	for key, value := range req.Filters {
		if allowedMangoFilter(key) && value != "" {
			values.Set(key, value)
		}
	}
	var body mangoResponse
	endpoint := "https://pianku.api.mgtv.com/rider/list/pcweb/v3?" + values.Encode()
	if err := requestJSON(ctx, http.MethodGet, endpoint, nil, map[string]string{"Referer": "https://www.mgtv.com"}, &body); err != nil {
		return listResult{}, err
	}
	items := make([]exploreItem, 0, len(body.Data.HitDocs))
	for _, item := range body.Data.HitDocs {
		if item.ClipID == "" || item.Title == "" {
			continue
		}
		poster := item.Image
		if poster == "" {
			poster = "https://1img.hitv.com/preview/sp_images/default.jpg"
		}
		var score *float64
		if value, err := strconv.ParseFloat(item.Score, 64); err == nil && value > 0 {
			score = &value
		}
		status := "ongoing"
		if strings.Contains(item.UpdateInfo, "全") {
			status = "finished"
		}
		items = append(items, exploreItem{Source: "mango", SourceItemID: item.ClipID, Title: item.Title, Year: item.Year, MediaType: sourceCategory(req.Category, "mango").MediaType, Category: req.Category, PosterURL: poster, SourceURL: "https://www.mgtv.com/h/" + url.PathEscape(item.ClipID) + ".html", Score: score, ScoreScaleMax: mangoMax(score), ScoreSource: scoreName(score, "mango"), Status: status, Subtitle: strings.Join(item.Kind, " · "), EpisodeText: item.UpdateInfo})
	}
	hasNext := len(items) == req.PageSize
	return listResult{Items: items, Pagination: pagination{Mode: "page", Page: req.Page, PageSize: req.PageSize, Total: body.Data.TotalCount, HasNext: &hasNext}}, nil
}

func allowedMangoFilter(key string) bool {
	for _, allowed := range []string{"chargeInfo", "sort", "kind", "edition", "area", "fitAge", "year", "feature"} {
		if key == allowed {
			return true
		}
	}
	return false
}

func (r *runtime) fetchMigu(req listRequest) (listResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	packID, display := miguChannel(req.Category)
	values := url.Values{"pageStart": {strconv.Itoa(req.Page)}, "pageNum": {strconv.Itoa(req.PageSize)}, "packId": {packID}, "contDisplayType": {display}, "copyrightTerminal": {"3"}}
	for key, value := range req.Filters {
		if allowedMiguFilter(key) && value != "" {
			values.Set(key, value)
		}
	}
	var body miguResponse
	endpoint := "https://jadeite.migu.cn/search/v3/category?" + values.Encode()
	if err := requestJSON(ctx, http.MethodGet, endpoint, nil, map[string]string{"Referer": "https://www.miguvideo.com/"}, &body); err != nil {
		return listResult{}, err
	}
	items := make([]exploreItem, 0, len(body.Body.Data))
	for _, item := range body.Body.Data {
		if item.PID == "" || item.Name == "" {
			continue
		}
		poster := item.H5Pics.HighV
		if poster == "" {
			poster = item.Pics.HighV
		}
		poster = strings.Replace(poster, "http://wapx.cmvideo.cn:8080", "https://wapx.cmvideo.cn", 1)
		var score *float64
		if value, err := strconv.ParseFloat(strings.TrimSpace(item.Score), 64); err == nil && value > 0 {
			score = &value
		}
		status := "ongoing"
		if strings.Contains(item.UpdateEP, "全") {
			status = "finished"
		}
		items = append(items, exploreItem{Source: "migu", SourceItemID: item.PID, Title: item.Name, Year: item.Year, MediaType: sourceCategory(req.Category, "migu").MediaType, Category: req.Category, PosterURL: poster, SourceURL: "https://www.miguvideo.com/p/detail/" + url.PathEscape(item.PID), Score: score, ScoreScaleMax: mangoMax(score), ScoreSource: scoreName(score, "migu"), Status: status, Subtitle: item.Subtitle, EpisodeText: item.UpdateEP})
	}
	hasNext := len(items) == req.PageSize
	return listResult{Items: items, Pagination: pagination{Mode: "page", Page: req.Page, PageSize: req.PageSize, HasNext: &hasNext}}, nil
}

func allowedMiguFilter(key string) bool {
	for _, allowed := range []string{"mediaType", "mediaArea", "mediaYear", "rankingType", "payType", "gender", "mediaAge"} {
		if key == allowed {
			return true
		}
	}
	return false
}

func miguChannel(category string) (string, string) {
	packs := map[string][2]string{
		"tv": {"1002581,1003861,1003863,1003866,1002601,1004761,1004121,1004641,1005521,1005261", "1001"},
		"movie": {"1002581,1002601,1003862,1003864,1003866,1004121,1003861,1004761,1004641", "1000"},
		"variety": {"1002581,1002601", "1005"}, "documentary": {"1002581,1002601", "1002"},
		"bangumi": {"1002581,1003861,1003863,1003866,1002601,1004761,1004121,1004641", "1007"}, "children": {"1002581,1002601", "601382"},
	}
	value := packs[category]
	return value[0], value[1]
}

func (r *runtime) fetchBangumi(req listRequest) (listResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var days []bangumiDay
	if err := requestJSONWithProxy(ctx, http.MethodGet, "https://api.bgm.tv/calendar", nil, map[string]string{"Referer": "https://api.bgm.tv/"}, r.bangumiProxy(), &days); err != nil {
		return listResult{}, err
	}
	wanted := 0
	if req.Category != "all" {
		for index, category := range sourceCategories["bangumi"] {
			if category.ID == req.Category {
				wanted = index
				break
			}
		}
	}
	all := make([]exploreItem, 0)
	for _, day := range days {
		if wanted != 0 && day.Weekday.ID != wanted {
			continue
		}
		for _, raw := range day.Items {
			if raw.ID == 0 {
				continue
			}
			name := raw.NameCN
			if name == "" {
				name = raw.Name
			}
			var score *float64
			if raw.Rating.Score > 0 {
				value := raw.Rating.Score
				score = &value
			}
			status := "ongoing"
			all = append(all, exploreItem{Source: "bangumi", SourceItemID: strconv.FormatInt(raw.ID, 10), Title: name, MediaType: "tv", Category: req.Category, PosterURL: raw.Images.Large, SourceURL: "https://bgm.tv/subject/" + strconv.FormatInt(raw.ID, 10), Score: score, ScoreScaleMax: mangoMax(score), ScoreSource: scoreName(score, "bangumi"), Status: status, Subtitle: raw.AirDate, EpisodeText: strconv.Itoa(raw.Eps) + " 集"})
		}
	}
	start := (req.Page - 1) * req.PageSize
	if start > len(all) {
		start = len(all)
	}
	end := start + req.PageSize
	if end > len(all) {
		end = len(all)
	}
	hasNext := end < len(all)
	return listResult{Items: all[start:end], Pagination: pagination{Mode: "page", Page: req.Page, PageSize: req.PageSize, Total: len(all), HasNext: &hasNext}}, nil
}

func (r *runtime) bangumiProxy() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	proxy, _ := r.config["bangumi_proxy"].(string)
	return strings.TrimSpace(proxy)
}

func sourceCategory(category, source string) category {
	for _, value := range sourceCategories[source] {
		if value.ID == category {
			return value
		}
	}
	return sourceCategories[source][0]
}

func mangoMax(score *float64) *float64 {
	if score == nil {
		return nil
	}
	value := 10.0
	return &value
}

func scoreName(score *float64, source string) string {
	if score == nil {
		return ""
	}
	return source
}

func (r *runtime) test(args map[string]interface{}) (interface{}, error) {
	source := "bilibili"
	if value, ok := args["source"].(string); ok && value != "" {
		source = value
	}
	if _, exists := sourceTitles[source]; !exists {
		return map[string]interface{}{"ok": false, "message": "未知来源"}, nil
	}
	if !r.sourceEnabled(source) {
		return map[string]interface{}{"ok": false, "message": "来源已停用"}, nil
	}
	category := sourceCategories[source][0].ID
	_, err := r.fetch(listRequest{Source: source, Category: category, Page: 1, PageSize: 1})
	if err != nil {
		return map[string]interface{}{"ok": false, "message": err.Error()}, nil
	}
	return map[string]interface{}{"ok": true, "message": sourceTitles[source] + " 接口连接正常"}, nil
}

func (r *runtime) recordHealth(source string, ok bool, latency int64, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h := r.health[source]
	h.LatencyMs = latency
	h.LastMessage = message
	if ok {
		h.Status = "ok"
		h.ConsecutiveFailures = 0
		h.LastSuccessAt = time.Now().UTC().Format(time.RFC3339)
	} else {
		h.Status = "degraded"
		h.ConsecutiveFailures++
		h.LastErrorAt = time.Now().UTC().Format(time.RFC3339)
	}
	r.health[source] = h
}
