package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	biliWebUA  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	biliAppUA  = "Mozilla/5.0 BiliDroid/6.73.0 (bbcallen@gmail.com) os/android model/SM-G9730 mobi_app/android build/6731100 channel/xiaomi innerVer/6731100 osVer/10 network/2"
	biliAppKey = "1d8b6e7d45233436"
	biliAppSec = "560c52ccd288fed045859ed18bffd973"
)

var (
	midRe       = regexp.MustCompile(`(?i)(?:space\.bilibili\.com/|bilibili\.com/(\d+)|[?&]mid=|[?&]up_id=)(\d+)`)
	midOnlyRe   = regexp.MustCompile(`^\d+$`)
	spaceNameRe = regexp.MustCompile(`^(.+?)的个人空间`)
)

type biliClient struct {
	http *http.Client
}

type biliVideo struct {
	Aid     int64
	BVID    string
	Title   string
	Author  string
	Pubdate int64
}

func newBiliClient() *biliClient {
	return &biliClient{http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *biliClient) setProxy(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		logf("代理无效")
		return
	}
	c.http.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
}

func (c *biliClient) do(req *http.Request) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}

func appSign(params map[string]string) url.Values {
	params["appkey"] = biliAppKey
	params["ts"] = strconv.FormatInt(time.Now().Unix(), 10)
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	q := url.Values{}
	var parts []string
	for _, k := range keys {
		q.Set(k, params[k])
		parts = append(parts, k+"="+url.QueryEscape(params[k]))
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + biliAppSec))
	q.Set("sign", hex.EncodeToString(sum[:]))
	return q
}

func (c *biliClient) card(mid int64) (name, face string, err error) {
	name, face = c.spaceHTML(mid)
	if name != "" {
		return name, face, nil
	}
	videos, _, _, vErr := c.videos(mid, 1, 0, nil)
	if vErr == nil && len(videos) > 0 && videos[0].Author != "" {
		return videos[0].Author, "", nil
	}
	if vErr != nil {
		return "", "", vErr
	}
	return "", "", fmt.Errorf("未找到该 UP")
}

func (c *biliClient) spaceHTML(mid int64) (name, face string) {
	req, err := http.NewRequest(http.MethodGet, "https://space.bilibili.com/"+strconv.FormatInt(mid, 10), nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", biliWebUA)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	body, err := c.do(req)
	if err != nil {
		return "", ""
	}
	html := string(body)
	if m := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(html); len(m) == 2 {
		if n := spaceNameRe.FindStringSubmatch(m[1]); len(n) == 2 {
			name = strings.TrimSpace(n[1])
		}
	}
	return name, face
}

func jsonInt64(raw json.RawMessage) int64 {
	s := strings.TrimSpace(string(raw))
	s = strings.Trim(s, `"`)
	if s == "" || s == "null" {
		return 0
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func clampFetchCount(n int) int {
	if n < 1 {
		return 20
	}
	if n > 100 {
		return 100
	}
	return n
}

func parseConfigInt(v interface{}, fallback int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return fallback
		}
		return int(n)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return fallback
		}
		return n
	default:
		return fallback
	}
}

func (c *biliClient) videos(mid int64, limit int, startAid int64, skip map[string]bool) ([]biliVideo, int64, bool, error) {
	limit = clampFetchCount(limit)
	out := make([]biliVideo, 0, limit)
	seen := map[string]bool{}
	aid := startAid
	cursor := startAid
	hasMore := false
	for pageNo := 0; pageNo < 25 && len(out) < limit; pageNo++ {
		// App 投稿接口 ps 写 50 也只会回 20，必须靠 aid 游标翻页。
		ps := 20
		page, lastAid, hasNext, err := c.videoPage(mid, ps, aid)
		if err != nil {
			if len(out) > 0 {
				return out, cursor, true, nil
			}
			return nil, cursor, false, err
		}
		kept := 0
		for _, v := range page {
			if v.Aid > 0 {
				cursor = v.Aid
			}
			if seen[v.BVID] || (skip != nil && skip[v.BVID]) {
				continue
			}
			seen[v.BVID] = true
			out = append(out, v)
			kept++
			if len(out) >= limit {
				break
			}
		}
		hasMore = hasNext || kept < len(page)
		if lastAid == 0 || lastAid == aid {
			hasMore = false
			break
		}
		aid = lastAid
		if len(out) >= limit {
			break
		}
		if !hasNext {
			hasMore = false
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	return out, cursor, hasMore, nil
}

func (c *biliClient) videoPage(mid int64, ps int, aid int64) ([]biliVideo, int64, bool, error) {
	params := map[string]string{
		"vmid": strconv.FormatInt(mid, 10),
		"ps":   strconv.Itoa(ps),
	}
	if aid > 0 {
		params["aid"] = strconv.FormatInt(aid, 10)
	}
	q := appSign(params)
	endpoint := "https://app.bilibili.com/x/v2/space/archive/cursor?" + q.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, false, err
	}
	req.Header.Set("User-Agent", biliAppUA)
	req.Header.Set("Accept", "application/json")
	raw, err := c.do(req)
	if err != nil {
		return nil, 0, false, err
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			HasNext bool `json:"has_next"`
			Item    []struct {
				Aid    json.RawMessage `json:"aid"`
				Param  json.RawMessage `json:"param"`
				BVID   string          `json:"bvid"`
				Title  string          `json:"title"`
				Author string          `json:"author"`
				Ctime  int64           `json:"ctime"`
			} `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, 0, false, err
	}
	if body.Code != 0 {
		return nil, 0, false, fmt.Errorf("B站 %d: %s", body.Code, body.Message)
	}
	out := make([]biliVideo, 0, len(body.Data.Item))
	var lastAid int64
	for _, v := range body.Data.Item {
		if strings.TrimSpace(v.BVID) == "" {
			continue
		}
		itemAid := jsonInt64(v.Aid)
		if itemAid == 0 {
			itemAid = jsonInt64(v.Param)
		}
		out = append(out, biliVideo{
			Aid:     itemAid,
			BVID:    v.BVID,
			Title:   v.Title,
			Author:  strings.TrimSpace(v.Author),
			Pubdate: v.Ctime,
		})
		if itemAid > 0 {
			lastAid = itemAid
		}
	}
	return out, lastAid, body.Data.HasNext, nil
}

func resolveURL(raw string) string {
	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return raw
	}
	req.Header.Set("User-Agent", biliWebUA)
	resp, err := client.Do(req)
	if err != nil {
		return raw
	}
	resp.Body.Close()
	if resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return raw
}

func parseMid(input string) (int64, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return 0, fmt.Errorf("请输入 mid 或空间链接")
	}
	if midOnlyRe.MatchString(s) {
		mid, err := strconv.ParseInt(s, 10, 64)
		if err != nil || mid <= 0 {
			return 0, fmt.Errorf("mid 无效")
		}
		return mid, nil
	}
	if strings.Contains(s, "b23.tv") || strings.Contains(s, "b23.wtf") {
		s = resolveURL(s)
	}
	if m := midRe.FindStringSubmatch(s); len(m) >= 3 {
		id := m[2]
		if id == "" {
			id = m[1]
		}
		mid, err := strconv.ParseInt(id, 10, 64)
		if err == nil && mid > 0 {
			return mid, nil
		}
	}
	if m := regexp.MustCompile(`(?:mid=|up_id=|space\.bilibili\.com/)(\d+)`).FindStringSubmatch(s); len(m) == 2 {
		mid, err := strconv.ParseInt(m[1], 10, 64)
		if err == nil && mid > 0 {
			return mid, nil
		}
	}
	return 0, fmt.Errorf("无法解析 UP，请填 mid 或空间链接")
}
