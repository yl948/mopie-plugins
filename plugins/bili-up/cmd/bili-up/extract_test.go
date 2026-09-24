package main

import "testing"

func TestExtractMovieName(t *testing.T) {
	cases := []struct{ in, out string }{
		{"打破港片票房纪录，现象级港产警匪片，至今难以超越！《寒战》", "寒战"},
		{"中国影史武侠片票房冠军，打破26年记录，碾压《卧虎藏龙》！《镖人：风起大漠》", "镖人：风起大漠"},
		{"如果你觉得上班很累，可以看看这部电影，到最后一定会治愈你", ""},
		{"票房暴涨1000倍的电影《牛来》，讲了个什么故事？", "牛来"},
		{"【阿奇】小镇惊悚片《第19排》：女人身陷无限轮回空间，上演空难版《恐怖游轮》", "第19排"},
		{"【阿奇】一男一女困在密闭房间，待满50天就能获奖500万/小众高概念电影《无暇的房间》", "无暇的房间"},
		{"【阿奇】小众高分末日灾难片，总有一部你没看过的", ""},
		{"疯狂动物城2：时隔十年全员回归", "疯狂动物城2"},
		{"「你的名字」新海诚神作回看", "你的名字"},
		{"【阿奇】核大战引发世界末日，幸存者躲在地下避难/一口气看完科幻惊悚剧末日地堡第三季", "末日地堡"},
	}
	for _, c := range cases {
		if got := extractMovieName(c.in); got != c.out {
			t.Fatalf("%q => %q, want %q", c.in, got, c.out)
		}
	}
}

func TestExtractCandidatesQuotedAndReference(t *testing.T) {
	cands := extractCandidates("碾压《卧虎藏龙》！《镖人：风起大漠》")
	if len(cands) != 2 {
		t.Fatalf("cands=%+v", cands)
	}
	if !cands[0].Reference || cands[0].Name != "卧虎藏龙" {
		t.Fatalf("ref %+v", cands[0])
	}
	if cands[1].Reference || cands[1].Name != "镖人：风起大漠" {
		t.Fatalf("main %+v", cands[1])
	}
}

func TestNamesMatch(t *testing.T) {
	if !namesMatch("寒战", "寒战") {
		t.Fatal("exact")
	}
	if !namesMatch("镖人：风起大漠", "镖人：风起大漠") {
		t.Fatal("punct")
	}
	if namesMatch("无", "无声") {
		t.Fatal("too short")
	}
}
