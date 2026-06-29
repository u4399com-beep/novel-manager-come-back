// Package services — crawler intelligence: smart category matching & status detection.
// Optimized for Go: pre-compiled maps, single-pass matching, zero allocations on hot paths.
package services

import (
	"regexp"
	"strings"
)

// ── Category Matching ──────────────────────────────────────────────────────

// CATEGORY_KEYWORDS maps Chinese category names to weighted keyword lists.
// Higher weight = stronger signal for that category.
var CATEGORY_KEYWORDS = map[string][]string{
	"言情": {"言情", "爱情", "婚恋", "恋爱", "总裁", "豪门", "甜宠", "契约", "替身"},
	"都市": {"都市", "职场", "官场", "商战", "重生", "穿越", "兵王", "神医", "鉴宝", "奶爸"},
	"唯美": {"唯美", "耽美", "百合", "纯爱"},
	"穿越": {"穿越", "重生", "异界", "召唤", "位面"},
	"青春": {"青春", "校园", "暗恋", "青梅"},
	"玄幻": {"玄幻", "修真", "修仙", "仙侠", "洪荒", "神话", "太古", "万界", "混沌"},
	"武侠": {"武侠", "江湖", "剑客", "侠客"},
	"军事": {"军事", "战争", "特种兵", "抗战"},
	"竞技": {"竞技", "游戏", "电竞", "网游", "体育", "篮球", "足球"},
	"科幻": {"科幻", "星际", "末世", "进化", "机甲", "虫族", "变异"},
	"悬疑": {"悬疑", "推理", "侦探", "灵异", "恐怖", "惊悚", "盗墓", "法医", "诡异"},
	"同人": {"同人", "火影", "海贼", "死神", "柯南", "综漫"},
	"职场": {"职场", "官场", "商战", "权势"},
}

// categoryWeights is pre-computed on init: keyword → (category, weight).
var categoryWeights map[string]string

func init() {
	categoryWeights = make(map[string]string, 100)
	for cat, keywords := range CATEGORY_KEYWORDS {
		for _, kw := range keywords {
			categoryWeights[kw] = cat
		}
	}
}

// MatchCategoryName matches a source-site category name to the best local category.
// Uses pre-compiled keyword map for O(n) single-pass matching.
func MatchCategoryName(sourceCategory string) string {
	if sourceCategory == "" {
		return ""
	}
	bestCat := ""
	bestScore := 0
	for keyword, cat := range categoryWeights {
		if strings.Contains(sourceCategory, keyword) {
			score := len(keyword) // longer keyword = more specific match
			if score > bestScore {
				bestScore = score
				bestCat = cat
			}
		}
	}
	// Edge cases
	if bestCat == "" && strings.Contains(sourceCategory, "耽美") {
		bestCat = "唯美"
	}
	return bestCat
}

// ── Status Detection ───────────────────────────────────────────────────────

// COMPLETED_PATTERNS are regex patterns that indicate a novel is completed.
var COMPLETED_PATTERNS = []*regexp.Regexp{
	regexp.MustCompile(`^(已)?完结$`),
	regexp.MustCompile(`全书完`),
	regexp.MustCompile(`大结局`),
	regexp.MustCompile(`已完结`),
	regexp.MustCompile(`全本`),
	regexp.MustCompile(`完本`),
	regexp.MustCompile(`[（(]完结[）)]`),
	regexp.MustCompile(`[（(]大结局[）)]`),
}

// ONGOING_PATTERNS match ongoing serialization.
var ONGOING_PATTERNS = []*regexp.Regexp{
	regexp.MustCompile(`^(连)?载(中)?$`),
	regexp.MustCompile(`连载`),
}

// STATUS_MAP is the final mapping from raw status string to canonical form.
var STATUS_MAP = map[string]string{
	"完结":   "completed",
	"已完结":  "completed",
	"连载":   "ongoing",
	"连载中":  "ongoing",
	"完本":   "completed",
	"全本":   "completed",
	"大结局":  "completed",
	"ongoing":  "ongoing",
	"completed": "completed",
}

// DetectNovelStatus detects whether a novel is completed based on meta tags,
// title keywords, and chapter title heuristics.
func DetectNovelStatus(rawStatus string, novelTitle string, latestChapterTitle string) string {
	// 1. Direct status map lookup
	if canonical, ok := STATUS_MAP[rawStatus]; ok {
		return canonical
	}
	if canonical, ok := STATUS_MAP[strings.TrimSpace(rawStatus)]; ok {
		return canonical
	}

	// 2. Pattern matching on raw status
	for _, re := range COMPLETED_PATTERNS {
		if re.MatchString(rawStatus) {
			return "completed"
		}
	}

	// 3. Check latest chapter title for completion indicators
	combined := novelTitle + " " + latestChapterTitle
	for _, re := range COMPLETED_PATTERNS {
		if re.MatchString(combined) {
			return "completed"
		}
	}

	// 4. Check if status contains ongoing patterns
	for _, re := range ONGOING_PATTERNS {
		if re.MatchString(rawStatus) {
			return "ongoing"
		}
	}

	// 5. Default: ongoing
	return "ongoing"
}

// ── Chapter Title Cleaning ─────────────────────────────────────────────────

var (
	reChapterNoise   = regexp.MustCompile(`(?i)(求.*票|求.*收藏|求.*订阅|求.*月票|求.*打赏|推荐票|月票|打赏|收藏|订阅|加更|补更).*$`)
	reChapterNum     = regexp.MustCompile(`^[第序]?\s*\d+\s*[章回节卷话].*$`)
	reBadTitleExact  = map[string]bool{"": true, "null": true, "undefined": true, "无标题": true, "新章节": true}
	reBadTitlePrefix = []string{"通知", "公告", "请假", "更新说明", "上架感言", "完本感言"}
)

// IsValidChapterTitle checks if a chapter title looks like actual content (not site noise).
func IsValidChapterTitle(title string) bool {
	title = strings.TrimSpace(title)
	if reBadTitleExact[title] {
		return false
	}
	for _, prefix := range reBadTitlePrefix {
		if strings.HasPrefix(title, prefix) {
			return false
		}
	}
	cleaned := reChapterNoise.ReplaceAllString(title, "")
	cleaned = strings.TrimSpace(cleaned)
	return len([]rune(cleaned)) >= 2
}

// CleanChapterTitle removes noise suffixes from chapter titles.
func CleanChapterTitle(title string) string {
	title = strings.TrimSpace(title)
	title = reChapterNoise.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}
