package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const tmdbSearchURL = "https://api.themoviedb.org/3/search/multi"

type tmdbHit struct {
	ID            int64
	Title         string
	OriginalTitle string
	Year          string
	Poster        string
	Rating        float64
	VoteCount     int
	URL           string
	MediaType     string
}

type tmdbClient struct {
	http   *http.Client
	apiKey string
	mu     sync.Mutex
	last   time.Time
	cache  map[string][]tmdbHit
}

func newTMDBClient() *tmdbClient {
	return &tmdbClient{
		http:  &http.Client{Timeout: 12 * time.Second},
		cache: map[string][]tmdbHit{},
	}
}

func (c *tmdbClient) configure(apiKey, proxy string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiKey = strings.TrimSpace(apiKey)
	c.cache = map[string][]tmdbHit{}
	client := &http.Client{Timeout: 12 * time.Second}
	if p := strings.TrimSpace(proxy); p != "" {
		if u, err := url.Parse(p); err == nil && u.Host != "" {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
		} else {
			logf("TMDB 代理无效")
		}
	}
	c.http = client
}

func tmdbPoster(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	raw := "https://image.tmdb.org/t/p/w500" + path
	return "/api/poster?url=" + url.QueryEscape(raw)
}

func tmdbPageURL(mediaType string, id int64) string {
	kind := "movie"
	if mediaType == "tv" {
		kind = "tv"
	}
	return fmt.Sprintf("https://www.themoviedb.org/%s/%d", kind, id)
}

func (c *tmdbClient) search(keyword string) ([]tmdbHit, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, nil
	}
	c.mu.Lock()
	if hits, ok := c.cache[keyword]; ok {
		c.mu.Unlock()
		return hits, nil
	}
	apiKey := c.apiKey
	client := c.http
	if wait := 250*time.Millisecond - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
	c.mu.Unlock()

	if apiKey == "" {
		return nil, fmt.Errorf("未配置 TMDB API Key")
	}
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("query", keyword)
	q.Set("language", "zh-CN")
	q.Set("include_adult", "false")
	req, err := http.NewRequest(http.MethodGet, tmdbSearchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mopie-bili-up/0.1")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Results []struct {
			MediaType     string  `json:"media_type"`
			ID            int64   `json:"id"`
			Title         string  `json:"title"`
			Name          string  `json:"name"`
			OriginalTitle string  `json:"original_title"`
			OriginalName  string  `json:"original_name"`
			ReleaseDate   string  `json:"release_date"`
			FirstAirDate  string  `json:"first_air_date"`
			PosterPath    string  `json:"poster_path"`
			VoteAverage   float64 `json:"vote_average"`
			VoteCount     int     `json:"vote_count"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	out := make([]tmdbHit, 0, len(payload.Results))
	for _, r := range payload.Results {
		if r.MediaType != "movie" && r.MediaType != "tv" {
			continue
		}
		if r.ID == 0 {
			continue
		}
		title := strings.TrimSpace(r.Title)
		if title == "" {
			title = strings.TrimSpace(r.Name)
		}
		original := strings.TrimSpace(r.OriginalTitle)
		if original == "" {
			original = strings.TrimSpace(r.OriginalName)
		}
		if title == "" {
			title = original
		}
		if title == "" {
			continue
		}
		date := r.ReleaseDate
		if date == "" {
			date = r.FirstAirDate
		}
		year := ""
		if len(date) >= 4 {
			year = date[:4]
		}
		out = append(out, tmdbHit{
			ID:            r.ID,
			Title:         title,
			OriginalTitle: original,
			Year:          year,
			Poster:        tmdbPoster(r.PosterPath),
			Rating:        r.VoteAverage,
			VoteCount:     r.VoteCount,
			URL:           tmdbPageURL(r.MediaType, r.ID),
			MediaType:     r.MediaType,
		})
	}
	c.mu.Lock()
	c.cache[keyword] = out
	c.mu.Unlock()
	return out, nil
}

func hitScore(name string, h tmdbHit) (int, bool) {
	nn := normName(name)
	if nn == "" {
		return 0, false
	}
	nt, no := normName(h.Title), normName(h.OriginalTitle)
	score := 0
	switch {
	case nt == nn || no == nn:
		score = 300
	case namesMatch(name, h.Title) || namesMatch(name, h.OriginalTitle):
		score = 100
	default:
		return 0, false
	}
	if h.MediaType == "movie" {
		score += 20
	}
	switch {
	case h.VoteCount > 1000:
		score += 15
	case h.VoteCount > 100:
		score += 8
	case h.VoteCount > 20:
		score += 3
	}
	return score, true
}

func pickTMDB(name string, hits []tmdbHit) *tmdbHit {
	bestI, best := -1, -1
	for i := range hits {
		score, ok := hitScore(name, hits[i])
		if !ok {
			continue
		}
		if score > best {
			best, bestI = score, i
		}
	}
	if bestI < 0 {
		return nil
	}
	return &hits[bestI]
}

func formatRate(v float64) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}
