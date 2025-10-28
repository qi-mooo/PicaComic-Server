package models

import "time"

// Comic 漫画基本信息
type Comic struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
	Cover       string    `json:"cover"`
	Tags        []string  `json:"tags"`
	Categories  []string  `json:"categories"`
	EpsCount    int       `json:"eps_count"`
	PagesCount  int       `json:"pages_count"`
	Type        string    `json:"type"` // picacg, ehentai, jm, etc.
	Time        time.Time `json:"time"`
	Size        int64     `json:"size"` // 文件大小（字节）
}

// ComicDetail 漫画详细信息
type ComicDetail struct {
	Comic
	Eps           []string `json:"eps"`            // 章节列表
	DownloadedEps []int    `json:"downloaded_eps"` // 已下载的章节索引
	Directory     string   `json:"directory"`      // 存储目录名
}

// DownloadTask 下载任务
type DownloadTask struct {
	ID              string    `json:"id"`
	ComicID         string    `json:"comic_id"`
	Title           string    `json:"title"`
	Author          string    `json:"author"`
	Type            string    `json:"type"`
	Cover           string    `json:"cover"`
	TotalPages      int       `json:"total_pages"`
	DownloadedPages int       `json:"downloaded_pages"`
	CurrentEp       int       `json:"current_ep"`
	Status          string    `json:"status"` // pending, downloading, paused, completed, error
	Error           string    `json:"error,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Description     string    `json:"description"`
	Extra           string    `json:"extra"`
	Tags            string    `json:"tags"`
}

// PicacgComic PicaComic 漫画信息
type PicacgComic struct {
	ID          string   `json:"_id"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	ThumbURL    string   `json:"thumb_url"`
	Tags        []string `json:"tags"`
	Categories  []string `json:"categories"`
	EpsCount    int      `json:"eps_count"`
	PagesCount  int      `json:"pages_count"`
	LikesCount  int      `json:"likes_count"`
}

// DownloadRequest 下载请求
type DownloadRequest struct {
	Type        string                 `json:"type" binding:"required"` // picacg, ehentai, etc.
	ComicID     string                 `json:"comic_id" binding:"required"`
	Title       string                 `json:"title"`
	Author      string                 `json:"author"`
	Cover       string                 `json:"cover"`
	Tags        map[string][]string    `json:"tags"`
	Description string                 `json:"description"`
	ComicInfo   string                 `json:"comic_info,omitempty"` // JSON 格式的漫画信息
	Eps         []int                  `json:"eps,omitempty"`        // 要下载的章节，为空则下载全部
	EpNames     []string               `json:"ep_names,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Keyword string `json:"keyword" binding:"required"`
	Sort    string `json:"sort"` // dd, da, ld, vd
	Page    int    `json:"page"`
}
