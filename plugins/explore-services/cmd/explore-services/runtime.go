package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func newRuntime() *runtime {
	return &runtime{cache: map[string]cacheEntry{}, health: map[string]sourceHealth{}, config: map[string]interface{}{}}
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
	r.dataDir, r.cacheDir, r.config = p.DataDir, p.CacheDir, p.Config
	if r.config == nil {
		r.config = map[string]interface{}{}
	}
	r.loadCache()
	return map[string]interface{}{
		"apiVersion": p.APIVersion,
		"name":       p.PluginName,
		"version":    "0.3.1-beta",
		"capabilities": []string{
			"explore.sources.list", "explore.list", "explore.health", "explore.connection.test",
		},
	}, nil
}

func (r *runtime) tool(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "explore.sources.list":
		return r.sources(), nil
	case "explore.health":
		return r.healthResult(), nil
	case "explore.list":
		var req listRequest
		data, _ := json.Marshal(args)
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
		return r.list(req)
	case "explore.connection.test":
		return r.test(args)
	default:
		return nil, fmt.Errorf("未知的工具: %s", name)
	}
}

func (r *runtime) sources() map[string]interface{} {
	items := make([]sourceDescriptor, 0, len(sourceOrder))
	for _, id := range sourceOrder {
		if !r.sourceEnabled(id) {
			continue
		}
		items = append(items, sourceDescriptor{
			ID:           id,
			Title:        sourceTitles[id],
			Enabled:      true,
			Categories:   sourceCategories[id],
			Sorts:        []option{{Value: "popular", Label: "热门"}, {Value: "updated", Label: "最近更新"}},
			FilterGroups: sourceFilterGroups[id],
		})
	}
	var defaultSource interface{}
	if len(items) > 0 {
		defaultSource = items[0].ID
	}
	return map[string]interface{}{"sources": items, "defaultSource": defaultSource}
}

func (r *runtime) sourceEnabled(source string) bool {
	value, configured := r.config[sourceConfig[source]]
	if !configured {
		return true
	}
	enabled, ok := value.(bool)
	return !ok || enabled
}

func (r *runtime) healthResult() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]sourceHealth, 0, len(sourceOrder))
	for _, source := range sourceOrder {
		if !r.sourceEnabled(source) {
			continue
		}
		h := r.health[source]
		if h.Status == "" {
			h.Status = "unknown"
		}
		h.Source = source
		items = append(items, h)
	}
	return map[string]interface{}{"sources": items}
}

func (r *runtime) list(req listRequest) (listResult, error) {
	if req.Source == "" {
		req.Source = "bilibili"
	}
	if _, ok := sourceTitles[req.Source]; !ok || !r.sourceEnabled(req.Source) {
		return listResult{}, fmt.Errorf("SOURCE_DISABLED: 来源不可用: %s", req.Source)
	}
	if !validCategory(req.Source, req.Category) {
		req.Category = sourceCategories[req.Source][0].ID
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 40 {
		req.PageSize = 24
	}
	if req.Filters == nil {
		req.Filters = map[string]string{}
	}
	key := cacheKey(req)
	now := time.Now()
	r.mu.RLock()
	cached, found := r.cache[key]
	r.mu.RUnlock()
	if found && now.Sub(cached.At) < 30*time.Minute {
		cached.Data.Cache = makeCacheInfo("fresh", cached.At, now)
		return cached.Data, nil
	}
	started := time.Now()
	data, err := r.fetch(req)
	latency := time.Since(started).Milliseconds()
	if err == nil {
		r.recordHealth(req.Source, true, latency, "")
		data.Cache = makeCacheInfo("miss", now, now)
		r.putCache(key, cacheEntry{At: now, Data: data})
		return data, nil
	}
	if found && now.Sub(cached.At) < 24*time.Hour {
		cached.Data.Cache = makeCacheInfo("stale", cached.At, now)
		cached.Data.Warning = &warning{Code: "UPSTREAM_UNAVAILABLE", Message: sourceTitles[req.Source] + " 更新失败，当前显示旧缓存", Retryable: true}
		r.recordHealth(req.Source, false, latency, err.Error())
		return cached.Data, nil
	}
	r.recordHealth(req.Source, false, latency, err.Error())
	return listResult{}, err
}

func validCategory(source, category string) bool {
	for _, item := range sourceCategories[source] {
		if item.ID == category {
			return true
		}
	}
	return false
}

func makeCacheInfo(state string, at, now time.Time) cacheInfo {
	return cacheInfo{State: state, FetchedAt: at.UTC().Format(time.RFC3339), ExpiresAt: at.Add(30 * time.Minute).UTC().Format(time.RFC3339), AgeSeconds: int64(now.Sub(at).Seconds())}
}

func cacheKey(req listRequest) string {
	data, _ := json.Marshal(req)
	return string(data) + "" + string(req.PageContext)
}

func (r *runtime) putCache(key string, entry cacheEntry) {
	r.mu.Lock()
	r.cache[key] = entry
	snapshot := make(map[string]cacheEntry, len(r.cache))
	for itemKey, value := range r.cache {
		if time.Since(value.At) < 24*time.Hour {
			snapshot[itemKey] = value
		}
	}
	r.cache = snapshot
	r.mu.Unlock()
	r.saveCache(snapshot)
}

func (r *runtime) loadCache() {
	if r.cacheDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(r.cacheDir, "explore.json"))
	if err != nil {
		return
	}
	var cache map[string]cacheEntry
	if json.Unmarshal(data, &cache) != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, entry := range cache {
		if time.Since(entry.At) < 24*time.Hour {
			r.cache[key] = entry
		}
	}
}

func (r *runtime) saveCache(cache map[string]cacheEntry) {
	if r.cacheDir == "" {
		return
	}
	data, err := json.Marshal(cache)
	if err != nil || os.MkdirAll(r.cacheDir, 0755) != nil {
		return
	}
	temp := filepath.Join(r.cacheDir, "explore.json.tmp")
	if os.WriteFile(temp, data, 0600) == nil {
		_ = os.Rename(temp, filepath.Join(r.cacheDir, "explore.json"))
	}
}
