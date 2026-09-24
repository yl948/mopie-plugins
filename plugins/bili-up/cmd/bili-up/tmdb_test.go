package main

import "testing"

func TestTMDBPoster(t *testing.T) {
	got := tmdbPoster("/abc.jpg")
	want := "/api/poster?url=https%3A%2F%2Fimage.tmdb.org%2Ft%2Fp%2Fw500%2Fabc.jpg"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if tmdbPoster("") != "" {
		t.Fatal("empty")
	}
}

func TestPickTMDB(t *testing.T) {
	hits := []tmdbHit{
		{ID: 1, Title: "无声的证言", OriginalTitle: "Silent Witness", MediaType: "tv", VoteCount: 5000},
		{ID: 2, Title: "寒战", OriginalTitle: "Cold War", MediaType: "movie", VoteCount: 800},
	}
	hit := pickTMDB("寒战", hits)
	if hit == nil || hit.ID != 2 {
		t.Fatal("zh title")
	}
	hit = pickTMDB("Cold War", hits)
	if hit == nil || hit.ID != 2 {
		t.Fatal("original title")
	}
	if pickTMDB("不存在的电影", hits) != nil {
		t.Fatal("miss")
	}
}

func TestPickTMDBPrefersExactMovie(t *testing.T) {
	hits := []tmdbHit{
		{ID: 1499071, Title: "寒战", MediaType: "movie", VoteCount: 12},
		{ID: 121281, Title: "寒战", OriginalTitle: "Cold War", MediaType: "movie", VoteCount: 900},
		{ID: 2750, Title: "无声", OriginalTitle: "Silent Witness", MediaType: "tv", VoteCount: 8000},
		{ID: 670000, Title: "无声", MediaType: "movie", VoteCount: 80},
	}
	if hit := pickTMDB("寒战", hits); hit == nil || hit.ID != 121281 {
		t.Fatalf("寒战 got %+v", hit)
	}
	if hit := pickTMDB("无声", hits); hit == nil || hit.ID != 670000 {
		t.Fatalf("无声 got %+v", hit)
	}
}
