package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Follow struct {
	Mid         int64  `json:"mid"`
	Name        string `json:"name"`
	Face        string `json:"face"`
	LastFetchAt int64  `json:"last_fetch_at"`
	LastError   string `json:"last_error"`
	Cursor      int64  `json:"cursor"`
	HasMore     bool   `json:"has_more"`
}

type Movie struct {
	Aid        int64  `json:"aid,omitempty"`
	BVID       string `json:"bvid"`
	Mid        int64  `json:"mid"`
	UpName     string `json:"up_name"`
	VideoTitle string `json:"video_title"`
	MovieName  string `json:"movie_name"`
	TmdbID     string `json:"tmdb_id"`
	DoubanID   string `json:"douban_id,omitempty"`
	Title      string `json:"title"`
	Year       string `json:"year"`
	Rate       string `json:"rate"`
	Cover      string `json:"cover"`
	URL        string `json:"url"`
	MediaType  string `json:"media_type"`
	Pubdate    int64  `json:"pubdate"`
	Matched    bool   `json:"matched"`
	Seen       bool   `json:"seen"`
}

type store struct {
	mu      sync.Mutex
	dir     string
	Follows []Follow `json:"follows"`
	Movies  []Movie  `json:"movies"`
}

func loadStore(dir string) *store {
	s := &store{dir: dir}
	_ = os.MkdirAll(dir, 0755)
	if data, err := os.ReadFile(filepath.Join(dir, "state.json")); err == nil {
		_ = json.Unmarshal(data, s)
	}
	if s.Follows == nil {
		s.Follows = []Follow{}
	}
	if s.Movies == nil {
		s.Movies = []Movie{}
	}
	for i := range s.Follows {
		if s.Follows[i].Cursor == 0 {
			s.Follows[i].HasMore = true
		}
	}
	return s
}

func (s *store) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, "state.json.tmp")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, "state.json"))
}

func (s *store) follow(mid int64) *Follow {
	for i := range s.Follows {
		if s.Follows[i].Mid == mid {
			return &s.Follows[i]
		}
	}
	return nil
}

func (s *store) movieByBVID(bvid string) *Movie {
	for i := range s.Movies {
		if s.Movies[i].BVID == bvid {
			return &s.Movies[i]
		}
	}
	return nil
}
