package main

import (
	"encoding/json"
	"testing"
)

func TestClampFetchCount(t *testing.T) {
	if got := clampFetchCount(0); got != 20 {
		t.Fatalf("0 => %d", got)
	}
	if got := clampFetchCount(1); got != 1 {
		t.Fatalf("1 => %d", got)
	}
	if got := clampFetchCount(50); got != 50 {
		t.Fatalf("50 => %d", got)
	}
	if got := clampFetchCount(101); got != 100 {
		t.Fatalf("101 => %d", got)
	}
}

func TestParseConfigInt(t *testing.T) {
	if got := parseConfigInt(float64(30), 20); got != 30 {
		t.Fatalf("float %d", got)
	}
	if got := parseConfigInt("40", 20); got != 40 {
		t.Fatalf("string %d", got)
	}
	if got := parseConfigInt(nil, 20); got != 20 {
		t.Fatalf("nil %d", got)
	}
}

func TestCursorParamAndHasNext(t *testing.T) {
	raw := []byte(`{"code":0,"message":"OK","data":{"has_next":true,"item":[{"bvid":"BV1xx","title":"《寒战》","author":"越哥","ctime":1,"param":"117251667663471"}]}}`)
	var body struct {
		Code int `json:"code"`
		Data struct {
			HasNext bool `json:"has_next"`
			Item    []struct {
				Param json.RawMessage `json:"param"`
				BVID  string          `json:"bvid"`
				Title string          `json:"title"`
			} `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if !body.Data.HasNext {
		t.Fatal("has_next")
	}
	if jsonInt64(body.Data.Item[0].Param) != 117251667663471 {
		t.Fatalf("param %d", jsonInt64(body.Data.Item[0].Param))
	}
}

func TestPaginate(t *testing.T) {
	start, end, pages := paginate(101, 1, 20)
	if start != 0 || end != 20 || pages != 6 {
		t.Fatalf("p1 %d %d %d", start, end, pages)
	}
	start, end, pages = paginate(101, 6, 20)
	if start != 100 || end != 101 || pages != 6 {
		t.Fatalf("p6 %d %d %d", start, end, pages)
	}
	start, end, pages = paginate(0, 1, 20)
	if start != 0 || end != 0 || pages != 1 {
		t.Fatalf("empty %d %d %d", start, end, pages)
	}
	start, end, pages = paginate(20, 0, 0)
	if start != 0 || end != 20 || pages != 1 {
		t.Fatalf("defaults %d %d %d", start, end, pages)
	}
}
