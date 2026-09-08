package main

import (
	"encoding/json"
	"sync"
	"time"
)

type runtime struct {
	mu       sync.RWMutex
	dataDir  string
	cacheDir string
	config   map[string]interface{}
	cache    map[string]cacheEntry
	health   map[string]sourceHealth
}

type cacheEntry struct {
	At   time.Time  `json:"at"`
	Data listResult `json:"data"`
}

type sourceHealth struct {
	Source              string `json:"source,omitempty"`
	Status              string `json:"status"`
	LastSuccessAt       string `json:"lastSuccessAt,omitempty"`
	LastErrorAt         string `json:"lastErrorAt,omitempty"`
	LatencyMs           int64  `json:"latencyMs"`
	ConsecutiveFailures int    `json:"consecutiveFailures"`
	LastMessage         string `json:"lastMessage,omitempty"`
}

type listRequest struct {
	Source      string            `json:"source"`
	Category    string            `json:"category"`
	Page        int               `json:"page"`
	PageSize    int               `json:"pageSize"`
	Sort        string            `json:"sort"`
	Filters     map[string]string `json:"filters"`
	PageContext json.RawMessage   `json:"pageContext,omitempty"`
}

type sourceDescriptor struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Enabled      bool          `json:"enabled"`
	Categories   []category    `json:"categories"`
	Sorts        []option      `json:"sorts"`
	FilterGroups []filterGroup `json:"filterGroups,omitempty"`
}

type category struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	MediaType string `json:"mediaType"`
}

type option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type filterGroup struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Key      string   `json:"key"`
	Category string   `json:"category,omitempty"`
	Options  []option `json:"options"`
}

type exploreItem struct {
	Source         string   `json:"source"`
	SourceItemID   string   `json:"sourceItemId"`
	SourceSeasonID string   `json:"sourceSeasonId,omitempty"`
	Title          string   `json:"title"`
	Year           string   `json:"year,omitempty"`
	MediaType      string   `json:"mediaType"`
	Category       string   `json:"category"`
	PosterURL      string   `json:"posterUrl,omitempty"`
	SourceURL      string   `json:"sourceUrl"`
	Score          *float64 `json:"score,omitempty"`
	ScoreScaleMax  *float64 `json:"scoreScaleMax,omitempty"`
	ScoreSource    string   `json:"scoreSource,omitempty"`
	EpisodeText    string   `json:"episodeText,omitempty"`
	Status         string   `json:"status"`
	Badges         []string `json:"badges,omitempty"`
	Subtitle       string   `json:"subtitle,omitempty"`
}

type listResult struct {
	Items      []exploreItem `json:"items"`
	Pagination pagination    `json:"pagination"`
	Cache      cacheInfo     `json:"cache"`
	Warning     *warning        `json:"warning,omitempty"`
	PageContext json.RawMessage `json:"pageContext,omitempty"`
}

type pagination struct {
	Mode     string `json:"mode"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"pageSize,omitempty"`
	Total    int    `json:"total,omitempty"`
	HasNext  *bool  `json:"hasNext"`
}

type cacheInfo struct {
	State      string `json:"state"`
	FetchedAt  string `json:"fetchedAt,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
	AgeSeconds int64  `json:"ageSeconds,omitempty"`
}

type warning struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type bilibiliResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List    []bilibiliItem  `json:"list"`
		HasNext json.RawMessage `json:"has_next"`
	} `json:"data"`
}

type bilibiliItem struct {
	MediaID    int64  `json:"media_id"`
	SeasonID   int64  `json:"season_id"`
	Title      string `json:"title"`
	Cover      string `json:"cover"`
	Score      string `json:"score"`
	Link       string `json:"link"`
	SeasonType int    `json:"season_type"`
	Badge      string `json:"badge"`
	IsFinish   int    `json:"is_finish"`
	IndexShow  string `json:"index_show"`
	Subtitle   string `json:"subTitle"`
}

type tencentResponse struct {
	Data struct {
		HasNextPage     bool            `json:"has_next_page"`
		NextPageContext json.RawMessage `json:"next_page_context"`
		ModuleList  []struct {
			ModuleData []struct {
				ModuleParams struct {
					HasNextPage string `json:"has_next_page"`
					TotalVideo  string `json:"total_video"`
				} `json:"module_params"`
				Items struct {
					Items []struct {
						ItemType string      `json:"item_type"`
						Params   tencentItem `json:"item_params"`
					} `json:"item_datas"`
				} `json:"item_data_lists"`
			} `json:"module_datas"`
		} `json:"module_list_datas"`
	} `json:"data"`
}

type tencentItem struct {
	CID        string `json:"cid"`
	Title      string `json:"title"`
	SeriesName string `json:"series_name"`
	Year       string `json:"year"`
	PicV       string `json:"new_pic_vt"`
	PicH       string `json:"new_pic_hz"`
	Subtitle   string `json:"sub_title"`
	TimeLong   string `json:"timelong"`
	Area       string `json:"area_name"`
	Latest     string `json:"latest_mark_label"`
}

type cctvResponse struct {
	Data struct {
		Total int        `json:"total"`
		List  []cctvItem `json:"list"`
	} `json:"data"`
}
type cctvItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Image string `json:"image"`
	FC    string `json:"fc"`
	Area  string `json:"area"`
	Year  string `json:"year"`
}

type mangoResponse struct {
	Data struct {
		HitDocs    []mangoItem `json:"hitDocs"`
		Total      int         `json:"total"`
		TotalCount int         `json:"totalCount"`
	} `json:"data"`
}
type mangoItem struct {
	ClipID     string   `json:"clipId"`
	Title      string   `json:"title"`
	Image      string   `json:"img"`
	Year       string   `json:"year"`
	Kind       []string `json:"kind"`
	UpdateInfo string   `json:"updateInfo"`
	Views      string   `json:"views"`
	Score      string   `json:"zhihuScore"`
	Schema     string   `json:"schema"`
}

type miguResponse struct {
	Body struct {
		Data []miguItem `json:"data"`
	} `json:"body"`
}
type miguItem struct {
	PID         string `json:"pID"`
	Name        string `json:"name"`
	Subtitle    string `json:"subTitle"`
	Year        string `json:"year"`
	Score       string `json:"score"`
	PublishTime string `json:"publishTime"`
	UpdateEP    string `json:"updateEP"`
	Pics        struct {
		HighV string `json:"highResolutionV"`
	} `json:"pics"`
	H5Pics struct {
		HighV string `json:"highResolutionV"`
	} `json:"h5pics"`
}

type bangumiDay struct {
	Weekday struct {
		ID int    `json:"id"`
		CN string `json:"cn"`
	} `json:"weekday"`
	Items []bangumiItem `json:"items"`
}
type bangumiItem struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	NameCN     string          `json:"name_cn"`
	AirDate    string          `json:"air_date"`
	AirWeekday json.RawMessage `json:"air_weekday"`
	Images     struct {
		Large string `json:"large"`
	} `json:"images"`
	Rating struct {
		Score float64 `json:"score"`
	} `json:"rating"`
	Eps int `json:"eps"`
}

var allowedFilters = map[string]bool{
	"release_date": true, "year": true, "sort": true, "season_status": true,
	"style_id": true, "season_month": true, "copyright": true, "is_finish": true,
	"area": true, "spoken_language_type": true, "season_version": true,
	"order": true, "producer_id": true,
}

var sourceOrder = []string{"bilibili", "tencent", "cctv", "mango", "migu", "bangumi"}
var sourceTitles = map[string]string{"bilibili": "Bilibili", "tencent": "腾讯视频", "cctv": "CCTV", "mango": "芒果TV", "migu": "咪咕视频", "bangumi": "Bangumi"}
var sourceConfig = map[string]string{"bilibili": "bilibili_enabled", "tencent": "tencent_enabled", "cctv": "cctv_enabled", "mango": "mango_enabled", "migu": "migu_enabled", "bangumi": "bangumi_enabled"}
var sourceCategories = map[string][]category{
	"bilibili": {{"tv", "电视剧", "tv"}, {"movie", "电影", "movie"}, {"documentary", "纪录片", "tv"}, {"bangumi", "番剧", "tv"}, {"guo", "国创", "tv"}, {"variety", "综艺", "tv"}},
	"tencent":  {{"tv", "电视剧", "tv"}, {"movie", "电影", "movie"}, {"variety", "综艺", "tv"}, {"documentary", "纪录片", "tv"}, {"bangumi", "动漫", "tv"}, {"children", "少儿", "tv"}},
	"cctv":     {{"tv", "电视剧", "tv"}, {"movie", "电影", "movie"}, {"animation", "动画片", "tv"}, {"documentary", "纪录片", "tv"}, {"special", "特别节目", "tv"}},
	"mango":    {{"tv", "电视剧", "tv"}, {"movie", "电影", "movie"}, {"bangumi", "动漫", "tv"}, {"children", "少儿", "tv"}, {"variety", "综艺", "tv"}, {"documentary", "纪录片", "tv"}, {"education", "教育", "tv"}},
	"migu":     {{"tv", "电视剧", "tv"}, {"movie", "电影", "movie"}, {"variety", "综艺", "tv"}, {"documentary", "纪实", "tv"}, {"bangumi", "动漫", "tv"}, {"children", "少儿", "tv"}},
	"bangumi":  {{"all", "全部", "tv"}, {"monday", "星期一", "tv"}, {"tuesday", "星期二", "tv"}, {"wednesday", "星期三", "tv"}, {"thursday", "星期四", "tv"}, {"friday", "星期五", "tv"}, {"saturday", "星期六", "tv"}, {"sunday", "星期日", "tv"}},
}

var sourceFilterGroups = map[string][]filterGroup{
	"bilibili": {
		{ID: "tv-area", Title: "地区", Key: "area", Category: "tv", Options: []option{{"1,6,7", "中国"}, {"2", "日本"}, {"3", "美国"}, {"4", "英国"}}},
		{ID: "movie-area", Title: "地区", Key: "area", Category: "movie", Options: []option{{"1", "中国大陆"}, {"6,7", "中国港台"}, {"3", "美国"}, {"2", "日本"}, {"8", "韩国"}, {"4", "英国"}}},
		{ID: "tv-type", Title: "类型", Key: "style_id", Category: "tv", Options: []option{{"10050", "剧情"}, {"10084", "情感"}, {"10021", "搞笑"}, {"10057", "悬疑"}, {"10080", "都市"}, {"10061", "家庭"}, {"10081", "古装"}, {"10033", "历史"}, {"10018", "奇幻"}, {"10079", "青春"}, {"10058", "战争"}, {"10078", "武侠"}, {"10023", "科幻"}}},
		{ID: "movie-type", Title: "类型", Key: "style_id", Category: "movie", Options: []option{{"10050", "剧情"}, {"10051", "喜剧"}, {"10052", "爱情"}, {"10053", "动作"}, {"10054", "恐怖"}, {"10023", "科幻"}, {"10055", "犯罪"}, {"10057", "悬疑"}, {"10018", "奇幻"}, {"10058", "战争"}, {"10059", "动画"}}},
		{ID: "bangumi-type", Title: "类型", Key: "season_version", Category: "bangumi", Options: []option{{"1", "正片"}, {"2", "电影"}, {"3", "其他"}}},
		{ID: "bangumi-area", Title: "地区", Key: "area", Category: "bangumi", Options: []option{{"2", "日本"}, {"3", "美国"}, {"1,4,5,6,7", "其他"}}},
		{ID: "finish-bangumi", Title: "状态", Key: "is_finish", Category: "bangumi", Options: []option{{"0", "连载"}, {"1", "完结"}}},
		{ID: "finish-guo", Title: "状态", Key: "is_finish", Category: "guo", Options: []option{{"0", "连载"}, {"1", "完结"}}},
		{ID: "year-release", Title: "年份", Key: "release_date", Category: "tv", Options: yearReleaseOptions},
		{ID: "year-movie", Title: "年份", Key: "release_date", Category: "movie", Options: yearReleaseOptions},
		{ID: "year-documentary", Title: "年份", Key: "release_date", Category: "documentary", Options: yearReleaseOptions},
		{ID: "year-bangumi", Title: "年份", Key: "year", Category: "bangumi", Options: yearRangeOptions},
		{ID: "year-guo", Title: "年份", Key: "year", Category: "guo", Options: yearRangeOptions},
	},
	"cctv": {
		{ID: "channel-documentary", Title: "频道", Key: "channel", Category: "documentary", Options: []option{{"CCTV-1综合,CCTV-1高清,CCTV-1综合高清", "CCTV-1"}, {"CCTV-9纪录,CCTV-9高清,CCTV-9纪录高清", "CCTV-9"}, {"CCTV-10科教,CCTV-10高清", "CCTV-10"}}},
		{ID: "tv-area", Title: "地区", Key: "area", Category: "tv", Options: []option{{"内地（大陆）", "内地"}, {"港澳台", "港澳台"}, {"欧美", "欧美"}, {"日韩", "日韩"}, {"其他", "其他"}}},
		{ID: "animation-area", Title: "地区", Key: "area", Category: "animation", Options: []option{{"内地（大陆）", "内地"}, {"港澳台", "港澳台"}, {"欧美", "欧美"}, {"日韩", "日韩"}, {"其他", "其他"}}},
		{ID: "tv-type", Title: "类型", Key: "sc", Category: "tv", Options: []option{{"谍战", "谍战"}, {"悬疑", "悬疑"}, {"刑侦", "刑侦"}, {"历史", "历史"}, {"古装", "古装"}, {"武侠", "武侠"}, {"战争", "战争"}, {"喜剧", "喜剧"}, {"都市", "都市"}}},
		{ID: "movie-type", Title: "类型", Key: "sc", Category: "movie", Options: []option{{"偶像", "偶像"}, {"古装", "古装"}, {"喜剧", "喜剧"}, {"惊悚", "惊悚"}, {"爱情", "爱情"}, {"战争", "战争"}, {"历史", "历史"}, {"传记", "传记"}}},
		{ID: "documentary-type", Title: "类型", Key: "sc", Category: "documentary", Options: []option{{"人文历史", "人文历史"}, {"人物", "人物"}, {"军事", "军事"}, {"探索", "探索"}, {"社会", "社会"}, {"自然", "自然"}, {"科技", "科技"}}},
		{ID: "year-tv", Title: "年份", Key: "year", Category: "tv", Options: yearOptions},
		{ID: "year-movie", Title: "年份", Key: "year", Category: "movie", Options: yearOptions},
		{ID: "letter", Title: "字母", Key: "fl", Options: letterOptions},
	},
	"migu": {
		{ID: "tv-type", Title: "类型", Key: "mediaType", Category: "tv", Options: []option{{"爱情", "爱情"}, {"古装", "古装"}, {"战争", "战争"}, {"悬疑", "悬疑"}, {"青春", "青春"}, {"都市", "都市"}, {"喜剧", "喜剧"}, {"家庭", "家庭"}, {"武侠", "武侠"}, {"科幻", "科幻"}}},
		{ID: "movie-type", Title: "类型", Key: "mediaType", Category: "movie", Options: []option{{"动作", "动作"}, {"喜剧", "喜剧"}, {"惊悚", "惊悚"}, {"悬疑", "悬疑"}, {"犯罪", "犯罪"}, {"战争", "战争"}, {"爱情", "爱情"}, {"动画", "动画"}, {"科幻", "科幻"}}},
		{ID: "tv-area", Title: "地区", Key: "mediaArea", Category: "tv", Options: []option{{"内地", "内地"}, {"香港地区", "香港"}, {"日本", "日本"}, {"美国", "美国"}, {"英国", "英国"}, {"韩国", "韩国"}, {"泰国", "泰国"}}},
		{ID: "movie-area", Title: "地区", Key: "mediaArea", Category: "movie", Options: []option{{"内地", "内地"}, {"中国香港", "香港"}, {"中国台湾", "台湾"}, {"美国", "美国"}, {"英国", "英国"}, {"日本", "日本"}, {"韩国", "韩国"}, {"泰国", "泰国"}}},
		{ID: "year-tv", Title: "年份", Key: "mediaYear", Category: "tv", Options: yearOptions},
		{ID: "year-movie", Title: "年份", Key: "mediaYear", Category: "movie", Options: yearOptions},
		{ID: "rank", Title: "排序", Key: "rankingType", Options: []option{{"0", "最热"}, {"1", "最新"}, {"2", "好评"}}},
	},
}

var yearOptions = []option{{"2025", "2025"}, {"2024", "2024"}, {"2023", "2023"}, {"2022", "2022"}, {"2021", "2021"}, {"2020", "2020"}, {"2019", "2019"}, {"2018", "2018"}, {"2017", "2017"}, {"2016", "2016"}}
var yearRangeOptions = []option{{"[2024,2025)", "2024"}, {"[2023,2024)", "2023"}, {"[2022,2023)", "2022"}, {"[2021,2022)", "2021"}, {"[2020,2021)", "2020"}, {"[2019,2020)", "2019"}, {"[2018,2019)", "2018"}}
var yearReleaseOptions = []option{{"[2024-01-01 00:00:00,2025-01-01 00:00:00]", "2024"}, {"[2023-01-01 00:00:00,2024-01-01 00:00:00)", "2023"}, {"[2022-01-01 00:00:00,2023-01-01 00:00:00)", "2022"}, {"[2021-01-01 00:00:00,2022-01-01 00:00:00)", "2021"}, {"[2020-01-01 00:00:00,2021-01-01 00:00:00)", "2020"}, {"[2019-01-01 00:00:00,2020-01-01 00:00:00)", "2019"}}
var letterOptions = []option{{"A", "A"}, {"B", "B"}, {"C", "C"}, {"D", "D"}, {"E", "E"}, {"F", "F"}, {"G", "G"}, {"H", "H"}, {"I", "I"}, {"J", "J"}, {"K", "K"}, {"L", "L"}, {"M", "M"}, {"N", "N"}, {"O", "O"}, {"P", "P"}, {"Q", "Q"}, {"R", "R"}, {"S", "S"}, {"T", "T"}, {"U", "U"}, {"V", "V"}, {"W", "W"}, {"X", "X"}, {"Y", "Y"}, {"Z", "Z"}}
