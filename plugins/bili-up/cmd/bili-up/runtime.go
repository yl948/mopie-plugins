package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const pluginVersion = "0.1.4"

type runtime struct {
	mu         sync.Mutex
	store      *store
	bili       *biliClient
	tmdb       *tmdbClient
	fetchCount int
	stopCh     chan struct{}
	busy       bool
}

func newRuntime() *runtime {
	return &runtime{
		bili:       newBiliClient(),
		tmdb:       newTMDBClient(),
		fetchCount: 20,
		stopCh:     make(chan struct{}),
	}
}

func (r *runtime) handle(method string, raw json.RawMessage) (interface{}, error) {
	switch method {
	case "initialize":
		return r.initialize(raw)
	case "health.check":
		return map[string]string{"status": "ok"}, nil
	case "tool.call":
		return r.callTool(raw)
	default:
		return nil, fmt.Errorf("未知的方法: %s", method)
	}
}

func (r *runtime) initialize(raw json.RawMessage) (interface{}, error) {
	var p struct {
		PluginName string                 `json:"pluginName"`
		APIVersion string                 `json:"apiVersion"`
		DataDir    string                 `json:"dataDir"`
		CacheDir   string                 `json:"cacheDir"`
		Config     map[string]interface{} `json:"config"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	dir := p.DataDir
	if dir == "" {
		dir = p.CacheDir
	}
	proxy := ""
	tmdbProxy := ""
	apiKey := ""
	fetchCount := 20
	if p.Config != nil {
		if v, ok := p.Config["proxy"].(string); ok {
			proxy = strings.TrimSpace(v)
		}
		if v, ok := p.Config["tmdb_proxy"].(string); ok {
			tmdbProxy = strings.TrimSpace(v)
		}
		if v, ok := p.Config["tmdb_api_key"].(string); ok {
			apiKey = strings.TrimSpace(v)
		}
		if _, ok := p.Config["fetch_count"]; ok {
			fetchCount = clampFetchCount(parseConfigInt(p.Config["fetch_count"], 20))
		}
	}
	if proxy == "" {
		proxy = tmdbProxy
	}
	r.fetchCount = fetchCount
	r.bili.setProxy(proxy)
	r.tmdb.configure(apiKey, tmdbProxy)
	r.store = loadStore(dir)
	go r.loop()
	logf("已启动 follows=%d movies=%d tmdb=%v fetch=%d", len(r.store.Follows), len(r.store.Movies), apiKey != "", r.fetchCount)
	return map[string]interface{}{
		"apiVersion": p.APIVersion,
		"name":       p.PluginName,
		"version":    pluginVersion,
		"capabilities": []string{
			"biliup.follows.list", "biliup.follows.add", "biliup.follows.remove",
			"biliup.refresh", "biliup.more", "biliup.movies.list",
		},
	}, nil
}

func (r *runtime) stop() {
	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}
}

func (r *runtime) loop() {
	t := time.NewTicker(6 * time.Hour)
	defer t.Stop()
	select {
	case <-time.After(45 * time.Second):
		if _, err := r.refreshAll(); err != nil {
			logf("定时刷新失败: %v", err)
		}
	case <-r.stopCh:
		return
	}
	for {
		select {
		case <-t.C:
			if _, err := r.refreshAll(); err != nil {
				logf("定时刷新失败: %v", err)
			}
		case <-r.stopCh:
			return
		}
	}
}

func (r *runtime) callTool(raw json.RawMessage) (interface{}, error) {
	var call struct {
		ToolName string                 `json:"toolName"`
		Args     map[string]interface{} `json:"args"`
	}
	if err := json.Unmarshal(raw, &call); err != nil {
		return nil, err
	}
	if r.store == nil {
		return nil, fmt.Errorf("插件未初始化")
	}
	result, err := r.tool(call.ToolName, call.Args)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"result": result}, nil
}

func (r *runtime) tool(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "biliup.follows.list":
		return r.listFollows(), nil
	case "biliup.follows.add":
		return r.addFollow(args)
	case "biliup.follows.remove":
		return r.removeFollow(args)
	case "biliup.refresh":
		return r.refreshAll()
	case "biliup.more":
		return r.more(args)
	case "biliup.movies.list":
		return r.listMovies(args), nil
	default:
		return nil, fmt.Errorf("未知的工具: %s", name)
	}
}

func (r *runtime) listFollows() map[string]interface{} {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	out := make([]Follow, len(r.store.Follows))
	copy(out, r.store.Follows)
	return map[string]interface{}{"follows": out}
}

func argString(args map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := args[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t)
				}
			case float64:
				return strconv.FormatInt(int64(t), 10)
			case json.Number:
				return t.String()
			}
		}
	}
	return ""
}

func argInt64(args map[string]interface{}, key string) int64 {
	v, ok := args[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	}
	return 0
}

func (r *runtime) addFollow(args map[string]interface{}) (interface{}, error) {
	input := argString(args, "input", "url", "mid")
	mid, err := parseMid(input)
	if err != nil {
		return nil, err
	}
	name, face, err := r.bili.card(mid)
	if err != nil {
		logf("资料接口不可用，先用 mid 关注: %v", err)
		name, face = strconv.FormatInt(mid, 10), ""
	}
	r.store.mu.Lock()
	if existing := r.store.follow(mid); existing != nil {
		existing.Name, existing.Face = name, face
		_ = r.store.save()
		copied := *existing
		r.store.mu.Unlock()
		go func() { _, _, _, _ = r.refreshMid(mid) }()
		return copied, nil
	}
	item := Follow{Mid: mid, Name: name, Face: face}
	r.store.Follows = append(r.store.Follows, item)
	_ = r.store.save()
	r.store.mu.Unlock()
	go func() { _, _, _, _ = r.refreshMid(mid) }()
	logf("关注 %s (%d)", name, mid)
	return item, nil
}

func (r *runtime) removeFollow(args map[string]interface{}) (interface{}, error) {
	mid := argInt64(args, "mid")
	if mid == 0 {
		parsed, err := parseMid(argString(args, "input", "mid"))
		if err != nil {
			return nil, err
		}
		mid = parsed
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	next := make([]Follow, 0, len(r.store.Follows))
	for _, f := range r.store.Follows {
		if f.Mid != mid {
			next = append(next, f)
		}
	}
	r.store.Follows = next
	kept := make([]Movie, 0, len(r.store.Movies))
	for _, m := range r.store.Movies {
		if m.Mid != mid {
			kept = append(kept, m)
		}
	}
	r.store.Movies = kept
	_ = r.store.save()
	return map[string]interface{}{"ok": true, "mid": mid}, nil
}

func (r *runtime) listMovies(args map[string]interface{}) map[string]interface{} {
	filterMid := argInt64(args, "mid")
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	candidates := make([]Movie, 0, len(r.store.Movies))
	for _, m := range r.store.Movies {
		if !m.Matched || m.TmdbID == "" || m.Cover == "" {
			continue
		}
		if filterMid > 0 && m.Mid != filterMid {
			continue
		}
		candidates = append(candidates, m)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Pubdate > candidates[j].Pubdate })
	seen := map[string]bool{}
	items := make([]Movie, 0, len(candidates))
	for _, m := range candidates {
		if seen[m.TmdbID] {
			continue
		}
		seen[m.TmdbID] = true
		items = append(items, m)
	}
	page := int(argInt64(args, "page"))
	size := int(argInt64(args, "page_size"))
	if page < 1 {
		page = 1
	}
	size = sizeIf(size)
	start, end, pages := paginate(len(items), page, size)
	hasMore := false
	if filterMid > 0 {
		if f := r.store.follow(filterMid); f != nil {
			hasMore = f.HasMore
		}
	} else {
		for i := range r.store.Follows {
			if r.store.Follows[i].HasMore {
				hasMore = true
				break
			}
		}
	}
	return map[string]interface{}{
		"items":     items[start:end],
		"page":      maxInt(page, 1),
		"page_size": sizeIf(size),
		"pages":     pages,
		"total":     len(items),
		"has_next":  page < pages || hasMore,
		"has_more":  hasMore,
	}
}

func sizeIf(size int) int {
	if size < 1 {
		return 20
	}
	if size > 50 {
		return 50
	}
	return size
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func paginate(n, page, size int) (start, end, pages int) {
	size = sizeIf(size)
	if page < 1 {
		page = 1
	}
	if n <= 0 {
		return 0, 0, 1
	}
	pages = (n + size - 1) / size
	start = (page - 1) * size
	if start > n {
		start = n
	}
	end = start + size
	if end > n {
		end = n
	}
	return start, end, pages
}

func (r *runtime) refreshAll() (map[string]int, error) {
	r.mu.Lock()
	if r.busy {
		r.mu.Unlock()
		return nil, fmt.Errorf("正在刷新")
	}
	r.busy = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.busy = false
		r.mu.Unlock()
	}()

	r.store.mu.Lock()
	mids := make([]int64, 0, len(r.store.Follows))
	for _, f := range r.store.Follows {
		mids = append(mids, f.Mid)
	}
	r.store.mu.Unlock()

	fetched, matched, unmatched := 0, 0, 0
	for i, mid := range mids {
		if i > 0 {
			time.Sleep(2 * time.Second)
		}
		f, m, u, err := r.refreshMid(mid)
		if err != nil {
			logf("拉取 %d 失败: %v", mid, err)
			continue
		}
		fetched += f
		matched += m
		unmatched += u
	}
	logf("刷新完成 fetched=%d matched=%d unmatched=%d", fetched, matched, unmatched)
	return map[string]int{"fetched": fetched, "matched": matched, "unmatched": unmatched}, nil
}

func (r *runtime) more(args map[string]interface{}) (map[string]int, error) {
	r.mu.Lock()
	if r.busy {
		r.mu.Unlock()
		return nil, fmt.Errorf("正在刷新")
	}
	r.busy = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.busy = false
		r.mu.Unlock()
	}()

	want := argInt64(args, "mid")
	r.store.mu.Lock()
	mids := make([]int64, 0, len(r.store.Follows))
	for _, f := range r.store.Follows {
		if want > 0 && f.Mid != want {
			continue
		}
		mids = append(mids, f.Mid)
	}
	r.store.mu.Unlock()
	if len(mids) == 0 {
		return map[string]int{"fetched": 0, "matched": 0, "unmatched": 0}, nil
	}

	fetched, matched, unmatched := 0, 0, 0
	for i, mid := range mids {
		if i > 0 {
			time.Sleep(2 * time.Second)
		}
		f, m, u, err := r.pullMid(mid, true)
		if err != nil {
			logf("续拉 %d 失败: %v", mid, err)
			continue
		}
		fetched += f
		matched += m
		unmatched += u
	}
	logf("续拉完成 fetched=%d matched=%d unmatched=%d", fetched, matched, unmatched)
	return map[string]int{"fetched": fetched, "matched": matched, "unmatched": unmatched}, nil
}

func (r *runtime) refreshMid(mid int64) (fetched, matched, unmatched int, err error) {
	return r.pullMid(mid, false)
}

func (r *runtime) pullMid(mid int64, onlyNew bool) (fetched, matched, unmatched int, err error) {
	r.store.mu.Lock()
	startAid := int64(0)
	skip := map[string]bool{}
	if onlyNew {
		if f := r.store.follow(mid); f != nil {
			startAid = f.Cursor
		}
		for _, m := range r.store.Movies {
			if m.Mid == mid && m.BVID != "" {
				skip[m.BVID] = true
			}
		}
	}
	r.store.mu.Unlock()

	videos, cursor, hasMore, err := r.bili.videos(mid, r.fetchCount, startAid, skip)
	if err != nil {
		r.store.mu.Lock()
		if f := r.store.follow(mid); f != nil {
			f.LastError = err.Error()
			f.LastFetchAt = time.Now().Unix()
			_ = r.store.save()
		}
		r.store.mu.Unlock()
		return 0, 0, 0, err
	}
	r.store.mu.Lock()
	upName := ""
	if f := r.store.follow(mid); f != nil {
		upName = f.Name
		f.LastError = ""
		f.LastFetchAt = time.Now().Unix()
		if onlyNew {
			if cursor > 0 {
				f.Cursor = cursor
			}
			f.HasMore = hasMore
		} else {
			if f.Cursor == 0 || (cursor > 0 && cursor < f.Cursor) {
				f.Cursor = cursor
			}
			if hasMore {
				f.HasMore = true
			}
		}
	}
	if len(videos) > 0 && videos[0].Author != "" {
		if f := r.store.follow(mid); f != nil && (f.Name == "" || f.Name == strconv.FormatInt(mid, 10)) {
			f.Name = videos[0].Author
		}
		upName = videos[0].Author
	}
	r.store.mu.Unlock()

	for _, v := range videos {
		fetched++
		r.store.mu.Lock()
		existing := r.store.movieByBVID(v.BVID)
		alreadyMatched := existing != nil && existing.Matched && existing.TmdbID != ""
		storedName := ""
		if existing != nil {
			storedName = existing.MovieName
		}
		r.store.mu.Unlock()
		if alreadyMatched && storedName != "" && storedName == extractMovieName(v.Title) {
			matched++
			continue
		}
		name, hit, searchErr := r.matchTitle(v.Title)
		item := Movie{
			Aid:        v.Aid,
			BVID:       v.BVID,
			Mid:        mid,
			UpName:     upName,
			VideoTitle: v.Title,
			MovieName:  name,
			Pubdate:    v.Pubdate,
			Seen:       true,
			MediaType:  "movie",
		}
		if name == "" {
			unmatched++
			r.upsertMovie(item)
			logf("未识别 %s", v.BVID)
			continue
		}
		if searchErr != nil && hit == nil {
			logf("TMDB 搜索失败 %s: %v", name, searchErr)
			unmatched++
			r.upsertMovie(item)
			continue
		}
		if hit == nil {
			unmatched++
			r.upsertMovie(item)
			logf("未匹配 %s", name)
			continue
		}
		item.Matched = true
		item.TmdbID = strconv.FormatInt(hit.ID, 10)
		item.Title = hit.Title
		item.Year = hit.Year
		item.Cover = hit.Poster
		item.URL = hit.URL
		item.MediaType = hit.MediaType
		item.Rate = formatRate(hit.Rating)
		matched++
		r.upsertMovie(item)
	}
	r.store.mu.Lock()
	_ = r.store.save()
	r.store.mu.Unlock()
	logf("拉取 %s mid=%d fetched=%d matched=%d unmatched=%d more=%v cursor=%d", upName, mid, fetched, matched, unmatched, hasMore, cursor)
	return fetched, matched, unmatched, nil
}

func (r *runtime) matchTitle(videoTitle string) (string, *tmdbHit, error) {
	cands := extractCandidates(videoTitle)
	if len(cands) == 0 {
		return "", nil, nil
	}
	var best *tmdbHit
	bestName := extractMovieName(videoTitle)
	bestScore := -1
	var lastErr error
	for _, cand := range cands {
		hits, searchErr := r.tmdb.search(cand.Name)
		if searchErr != nil {
			lastErr = searchErr
			logf("TMDB 搜索失败 %s: %v", cand.Name, searchErr)
			continue
		}
		h := pickTMDB(cand.Name, hits)
		if h == nil {
			continue
		}
		score, ok := hitScore(cand.Name, *h)
		if !ok {
			continue
		}
		if cand.Quoted {
			score += 50
		}
		if cand.Reference {
			score -= 80
		}
		if score > bestScore {
			cp := *h
			best = &cp
			bestName = cand.Name
			bestScore = score
		}
	}
	if best == nil {
		return bestName, nil, lastErr
	}
	return bestName, best, nil
}

func (r *runtime) upsertMovie(item Movie) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if existing := r.store.movieByBVID(item.BVID); existing != nil {
		*existing = item
		return
	}
	r.store.Movies = append(r.store.Movies, item)
}
