package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"

	"pica-comic-server/models"
	"pica-comic-server/services"

	"github.com/gin-gonic/gin"
)

// AddDownloadTask 添加下载任务
func AddDownloadTask(c *gin.Context) {
	var req models.DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = req.ComicID
	}

	req.Author = strings.TrimSpace(req.Author)
	req.Description = strings.TrimSpace(req.Description)
	req.Cover = strings.TrimSpace(req.Cover)

	if req.Tags == nil {
		req.Tags = map[string][]string{}
	}

	if req.Extra == nil {
		req.Extra = map[string]interface{}{}
	}

	// ⚠️ 警告：旧的 API 模式已废弃，所有下载都应使用直接下载模式
	// 这样可以避免跨域、证书验证等问题
	c.JSON(http.StatusBadRequest, gin.H{
		"error": "此 API 已废弃，请使用 POST /api/download/direct 提交直接下载任务（客户端拦截 URL）",
	})
	return

	// 下面的代码保留以防需要
	/*
		if req.Type == "picacg" {
			req.Extra["comic_id"] = req.ComicID
			req.Extra["eps"] = req.Eps
			req.Extra["ep_names"] = req.EpNames
		}
	*/

	if len(req.EpNames) > 0 {
		for i, name := range req.EpNames {
			req.EpNames[i] = strings.TrimSpace(name)
		}
	}

	if len(req.Eps) == 0 && len(req.EpNames) > 0 {
		req.Eps = make([]int, len(req.EpNames))
		for i := range req.EpNames {
			req.Eps[i] = i + 1
		}
	}

	if len(req.Eps) > 0 {
		type epSelection struct {
			index int
			name  string
		}

		selections := make([]epSelection, 0, len(req.Eps))
		for i, ep := range req.Eps {
			var name string
			if i < len(req.EpNames) {
				name = req.EpNames[i]
			}
			selections = append(selections, epSelection{index: ep, name: name})
		}

		deduped := make([]epSelection, 0, len(selections))
		seen := make(map[int]struct{}, len(selections))
		for _, item := range selections {
			if item.index <= 0 {
				continue
			}
			if _, exists := seen[item.index]; exists {
				continue
			}
			seen[item.index] = struct{}{}
			deduped = append(deduped, item)
		}

		sort.Slice(deduped, func(i, j int) bool {
			return deduped[i].index < deduped[j].index
		})

		eps := make([]int, len(deduped))
		names := make([]string, len(deduped))
		for i, item := range deduped {
			eps[i] = item.index
			if item.name != "" {
				names[i] = item.name
			} else {
				names[i] = fmt.Sprintf("第 %d 章", item.index)
			}
		}

		req.Eps = eps
		req.EpNames = names
	} else {
		req.EpNames = nil
	}

	task, err := services.GetDownloadManager().AddDownloadTask(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "任务添加成功",
		"task":    task,
	})
}

// GetDownloadQueue 获取下载队列
func GetDownloadQueue(c *gin.Context) {
	queue := services.GetDownloadManager().GetDownloadQueue()

	c.JSON(http.StatusOK, gin.H{
		"queue":          queue,
		"total":          len(queue),
		"is_downloading": services.GetDownloadManager().IsDownloading(),
	})
}

// StartDownload 开始/继续下载
func StartDownload(c *gin.Context) {
	if err := services.GetDownloadManager().Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "下载已开始",
	})
}

// PauseDownload 暂停下载
func PauseDownload(c *gin.Context) {
	services.GetDownloadManager().Pause()

	c.JSON(http.StatusOK, gin.H{
		"message": "下载已暂停",
	})
}

// CancelDownload 取消下载任务
func CancelDownload(c *gin.Context) {
	id := c.Param("id")

	if err := services.GetDownloadManager().CancelTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "任务已取消",
	})
}

// DirectDownloadRequest 直接下载请求（方案2 fallback）
type DirectDownloadRequest struct {
	ComicID     string              `json:"comic_id"`
	Type        string              `json:"type"` // "picacg", "jm", "eh"
	Title       string              `json:"title"`
	Cover       string              `json:"cover"`
	Author      string              `json:"author"`
	Description string              `json:"description"`
	Tags        map[string][]string `json:"tags"`
	Episodes    []DirectEpisode     `json:"episodes"`
}

type DirectEpisode struct {
	Order    int               `json:"order"`     // 章节序号 (1-based)
	Name     string            `json:"name"`      // 章节名称
	PageURLs []string          `json:"page_urls"` // 图片URL列表
	Headers  map[string]string `json:"headers"`   // HTTP请求头（客户端提供）
}

// SubmitDirectDownload 提交直接下载任务（客户端已获取URL）
func SubmitDirectDownload(c *gin.Context) {
	// 先读取原始请求体用于调试
	bodyBytes, _ := c.GetRawData()
	log.Printf("[DirectDownload] 收到请求，Body长度: %d bytes", len(bodyBytes))
	
	// 重新设置请求体供后续使用
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	
	var req DirectDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[DirectDownload] ❌ JSON解析失败: %v", err)
		log.Printf("[DirectDownload] 请求Body: %s", string(bodyBytes))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	log.Printf("[DirectDownload] ✓ JSON解析成功")
	log.Printf("[DirectDownload] ComicID: %s, Type: %s, Episodes: %d", req.ComicID, req.Type, len(req.Episodes))
	
	if req.ComicID == "" || len(req.Episodes) == 0 {
		log.Printf("[DirectDownload] ❌ 参数验证失败: ComicID=%s, Episodes=%d", req.ComicID, len(req.Episodes))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少必要参数：comic_id 和 episodes",
		})
		return
	}

	// 调用下载管理器直接下载
	taskID, err := services.GetDownloadManager().SubmitDirectDownload(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "提交下载任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "下载任务已提交（直接下载模式）",
		"task_id": taskID,
	})
}

// ImportComic 导入已下载的漫画（从客户端上传）
func ImportComic(c *gin.Context) {
	log.Println("[导入API] 收到漫画导入请求")

	// 1. 解析表单数据
	err := c.Request.ParseMultipartForm(100 << 20) // 100MB max
	if err != nil {
		log.Printf("[导入API] 解析表单失败: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析表单失败: " + err.Error()})
		return
	}

	// 2. 获取元数据
	comicID := c.PostForm("comic_id")
	title := c.PostForm("title")
	comicType := c.PostForm("type")
	author := c.PostForm("author")
	description := c.PostForm("description")
	cover := c.PostForm("cover")

	log.Printf("[导入API] 元数据: ID=%s, 标题=%s, 类型=%s, 作者=%s\n", comicID, title, comicType, author)

	if comicID == "" || title == "" || comicType == "" {
		log.Println("[导入API] 缺少必要字段")
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要字段"})
		return
	}

	// 记录上传的文件数量
	if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
		files := c.Request.MultipartForm.File["files"]
		log.Printf("[导入API] 收到 %d 个上传文件\n", len(files))

		// 列出前10个文件名用于调试
		for i, file := range files {
			if i < 10 {
				log.Printf("[导入API] 文件 %d: %s (%.2f KB)\n", i+1, file.Filename, float64(file.Size)/1024)
			}
		}
		if len(files) > 10 {
			log.Printf("[导入API] ... 还有 %d 个文件\n", len(files)-10)
		}
	} else {
		log.Println("[导入API] ⚠️ 警告: 没有收到任何文件！")
	}

	// 3. 调用服务层导入漫画
	dm := services.GetDownloadManager()
	err = dm.ImportComicFromClient(c.Request, comicID, title, comicType, author, description, cover)
	if err != nil {
		log.Printf("[导入API] 导入失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导入失败: " + err.Error()})
		return
	}

	log.Printf("[导入API] ✅ 漫画 '%s' 导入成功\n", title)
	c.JSON(http.StatusOK, gin.H{
		"message":  "导入成功",
		"comic_id": comicID,
	})
}
