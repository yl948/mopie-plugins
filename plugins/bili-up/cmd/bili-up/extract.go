package main

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	bookRe = regexp.MustCompile(`《([^》]+)》`)
	cnRe   = regexp.MustCompile(`「([^」]+)」`)
	cnRe2  = regexp.MustCompile(`『([^』]+)』`)
	dqRe   = regexp.MustCompile(`“([^”]+)”`)
	enRe   = regexp.MustCompile(`"([^"]{2,40})"`)
	tagRe  = regexp.MustCompile(`^\s*(?:【[^】]{1,30}】\s*)+`)
	seasonRe = regexp.MustCompile(`(?i)[\s\-]*(?:S\d+E\d+(?:-E?\d+)?|第[0-9一二三四五六七八九十百]+季.*|大结局|\d+\+\d+集|\d+集)\s*$`)
	yearGenreRe = regexp.MustCompile(`^(?:\d{4}年)?(?:一口气看完)?(?:小众)?(?:高概念)?(?:科幻|惊悚|动作|爱情|犯罪|悬疑|恐怖|喜剧|文艺|古装|怪兽)*(?:衍生)?(?:剧集|电影|片|剧)?`)
	junkRe = regexp.MustCompile(`这部|看完|年度|最新|高分|必看|影评|解说|解析|推荐|盘点|合集|没看过|时隔|全员回归`)
	refBeforeRe = regexp.MustCompile(`(?:碾压|致敬|翻拍|类似|好比|改编|对比|空难版|台湾版|日本版|韩国版|美版|港版|英版|国版|.版)$`)
)

type nameCand struct {
	Name      string
	Quoted    bool
	Reference bool
}

func extractMovieName(title string) string {
	cands := extractCandidates(title)
	for i := len(cands) - 1; i >= 0; i-- {
		if cands[i].Quoted && !cands[i].Reference {
			return cands[i].Name
		}
	}
	for i := len(cands) - 1; i >= 0; i-- {
		if cands[i].Quoted {
			return cands[i].Name
		}
	}
	if len(cands) > 0 {
		return cands[len(cands)-1].Name
	}
	return ""
}

func extractCandidates(title string) []nameCand {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	var out []nameCand
	seen := map[string]bool{}
	add := func(name string, quoted, ref bool) {
		name = strings.TrimSpace(name)
		name = seasonRe.ReplaceAllString(name, "")
		name = strings.TrimSpace(name)
		if !quoted && strings.ContainsAny(name, "，。！？;；") {
			return
		}
		if !goodTitle(name) {
			return
		}
		key := normName(name)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, nameCand{Name: name, Quoted: quoted, Reference: ref})
	}

	addQuoted := func(re *regexp.Regexp) {
		for _, m := range re.FindAllStringSubmatchIndex(title, -1) {
			if len(m) < 4 || m[2] < 0 {
				continue
			}
			name := title[m[2]:m[3]]
			before := strings.TrimSpace(title[:m[0]])
			runes := []rune(before)
			tail := before
			if n := len(runes); n > 8 {
				tail = string(runes[n-8:])
			}
			add(name, true, refBeforeRe.MatchString(tail))
		}
	}
	addQuoted(bookRe)
	addQuoted(cnRe)
	addQuoted(cnRe2)
	addQuoted(dqRe)
	addQuoted(enRe)
	if len(out) > 0 {
		return out
	}

	s := strings.TrimSpace(tagRe.ReplaceAllString(title, ""))
	for _, part := range splitTitleParts(s) {
		part = strings.TrimSpace(part)
		part = yearGenreRe.ReplaceAllString(part, "")
		part = seasonRe.ReplaceAllString(part, "")
		add(part, false, false)
	}
	return out
}

func splitTitleParts(s string) []string {
	s = strings.NewReplacer("｜", "/", "|", "/", "：", ":").Replace(s)
	var parts []string
	for _, p := range strings.Split(s, "/") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i := strings.Index(p, ":"); i > 0 {
			left := strings.TrimSpace(p[:i])
			right := strings.TrimSpace(p[i+1:])
			if left != "" {
				parts = append(parts, left)
			}
			if right != "" {
				parts = append(parts, right)
			}
			continue
		}
		parts = append(parts, p)
	}
	return parts
}

func goodTitle(s string) bool {
	s = strings.TrimSpace(s)
	n := len([]rune(s))
	if n < 2 || n > 30 {
		return false
	}
	if junkRe.MatchString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func normName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func namesMatch(extracted, other string) bool {
	a, b := normName(extracted), normName(other)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if len([]rune(extracted)) < 2 {
		return false
	}
	return strings.Contains(b, a) || strings.Contains(a, b)
}
