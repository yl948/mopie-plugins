package main

import (
	"encoding/json"
	"testing"
)

func TestParseBoolSupportsBilibiliShapes(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want bool
	}{{`true`, true}, {`false`, false}, {`1`, true}, {`0`, false}} {
		if got := parseBool(json.RawMessage(test.raw)); got != test.want {
			t.Fatalf("parseBool(%s) = %v, want %v", test.raw, got, test.want)
		}
	}
}

func TestNormalizeBilibiliMediaType(t *testing.T) {
	item := bilibiliItem{MediaID: 1, SeasonID: 2, Title: "示例", SeasonType: 2, IsFinish: 1, Score: "9.5"}
	got := normalizeBilibili(item, "bangumi")
	if got.MediaType != "movie" || got.Status != "finished" || got.Score == nil || *got.Score != 9.5 {
		t.Fatalf("unexpected normalized item: %+v", got)
	}
}

func TestCacheKeySeparatesFilters(t *testing.T) {
	first := cacheKey(listRequest{Source: "bilibili", Category: "bangumi", Page: 1, PageSize: 20, Filters: map[string]string{"area": "2"}})
	second := cacheKey(listRequest{Source: "bilibili", Category: "bangumi", Page: 1, PageSize: 20, Filters: map[string]string{"area": "3"}})
	if first == second {
		t.Fatal("different filters must produce different cache keys")
	}
}
