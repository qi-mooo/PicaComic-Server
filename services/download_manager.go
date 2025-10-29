package services

import (
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"pica-comic-server/models"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

var (
	downloadManager *DownloadManager
	once            sync.Once
)

// DownloadManager 下载管理器
type DownloadManager struct {
	mu            sync.RWMutex
	downloadPath  string
	db            *sql.DB
	queue         []*models.DownloadTask
	isDownloading bool
	currentTask   *models.DownloadTask
	stopChan      chan bool
	minDiskSpace  int64
}

// InitDownloadManager 初始化下载管理器
func InitDownloadManager(downloadPath string) error {
	var initErr error
	once.Do(func() {
		downloadManager = &DownloadManager{
			downloadPath: downloadPath,
			queue:        make([]*models.DownloadTask, 0),
			stopChan:     make(chan bool, 1), // 缓冲为1，避免 Pause() 阻塞
			minDiskSpace: 200 * 1024 * 1024,
		}
		initErr = downloadManager.init()
	})
	return initErr
}

// GetDownloadManager 获取下载管理器实例
func GetDownloadManager() *DownloadManager {
	return downloadManager
}

func (dm *DownloadManager) init() error {
	if err := dm.ensureDownloadDir(); err != nil {
		return err
	}

	// 打开数据库
	dbPath := filepath.Join(dm.downloadPath, "download.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	dm.db = db

	// 创建表
	if err := dm.createTables(); err != nil {
		return fmt.Errorf("创建数据表失败: %w", err)
	}

	// 运行数据库迁移
	if err := dm.migrateTables(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	// 加载未完成的任务
	if err := dm.loadPendingTasks(); err != nil {
		return fmt.Errorf("加载待处理任务失败: %w", err)
	}

	return nil
}

// migrateTables 执行数据库迁移
func (dm *DownloadManager) migrateTables() error {
	// 检查 comics 表是否有 detail_url 列
	var columnExists bool
	err := dm.db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('comics') 
		WHERE name='detail_url'
	`).Scan(&columnExists)

	if err != nil {
		return fmt.Errorf("检查 detail_url 列失败: %w", err)
	}

	// 如果列不存在，添加它
	if !columnExists {
		log.Println("[Migration] 添加 detail_url 列到 comics 表")
		_, err = dm.db.Exec(`ALTER TABLE comics ADD COLUMN detail_url TEXT`)
		if err != nil {
			return fmt.Errorf("添加 detail_url 列失败: %w", err)
		}
		log.Println("[Migration] ✓ detail_url 列添加成功")
	}

	return nil
}

// ensureDownloadDir 确保下载目录存在
func (dm *DownloadManager) ensureDownloadDir() error {
	if err := os.MkdirAll(dm.downloadPath, 0755); err != nil {
		return fmt.Errorf("创建下载目录失败: %w", err)
	}
	return nil
}

// ensureDiskSpace 检查磁盘空间是否充足
func (dm *DownloadManager) ensureDiskSpace() error {
	// 这里可以添加实际的磁盘空间检查逻辑
	// 暂时只返回 nil
	return nil
}

func (dm *DownloadManager) createTables() error {
	// 下载的漫画表
	_, err := dm.db.Exec(`
		CREATE TABLE IF NOT EXISTS comics (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			author TEXT,
			description TEXT,
			cover TEXT,
			tags TEXT,
			categories TEXT,
			eps_count INTEGER,
			pages_count INTEGER,
			type TEXT,
			time INTEGER,
			size INTEGER,
			directory TEXT,
			eps TEXT,
			downloaded_eps TEXT,
			detail_url TEXT
		)
	`)
	if err != nil {
		return err
	}

	// 下载任务表
	_, err = dm.db.Exec(`
		CREATE TABLE IF NOT EXISTS download_tasks (
			id TEXT PRIMARY KEY,
			comic_id TEXT NOT NULL,
			title TEXT NOT NULL,
			type TEXT NOT NULL,
			cover TEXT,
			total_pages INTEGER,
			downloaded_pages INTEGER,
			current_ep INTEGER,
			status TEXT,
			error TEXT,
			created_at INTEGER,
			updated_at INTEGER,
			description TEXT,
			extra TEXT,
			tags TEXT,
			author TEXT
		)
	`)
	return err
}

func (dm *DownloadManager) loadPendingTasks() error {
	rows, err := dm.db.Query(`
		SELECT id, comic_id, title, type, cover, total_pages, downloaded_pages, 
		       current_ep, status, error, created_at, updated_at,
		       COALESCE(description, ''), COALESCE(extra, ''), COALESCE(tags, ''), COALESCE(author, '')
		FROM download_tasks
		WHERE status IN ('pending', 'downloading', 'paused')
		ORDER BY created_at
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		task := &models.DownloadTask{}
		var createdAt, updatedAt int64
		err := rows.Scan(
			&task.ID, &task.ComicID, &task.Title, &task.Type, &task.Cover,
			&task.TotalPages, &task.DownloadedPages, &task.CurrentEp,
			&task.Status, &task.Error, &createdAt, &updatedAt,
			&task.Description, &task.Extra, &task.Tags, &task.Author,
		)
		if err != nil {
			continue
		}
		task.CreatedAt = time.Unix(createdAt, 0)
		task.UpdatedAt = time.Unix(updatedAt, 0)
		dm.queue = append(dm.queue, task)
	}

	return nil
}

// AddDownloadTask 添加下载任务
func (dm *DownloadManager) AddDownloadTask(req models.DownloadRequest) (*models.DownloadTask, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if err := dm.ensureDiskSpace(); err != nil {
		return nil, err
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	extraJSON, _ := json.Marshal(req.Extra)

	task := &models.DownloadTask{
		ID:              uuid.New().String(),
		ComicID:         req.ComicID,
		Type:            req.Type,
		Status:          "pending",
		DownloadedPages: 0,
		CurrentEp:       0,
		Title:           req.Title,
		Author:          req.Author,
		Cover:           req.Cover,
		TotalPages:      len(req.Eps),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Description:     req.Description,
		Extra:           string(extraJSON),
		Tags:            string(tagsJSON),
	}

	// 保存到数据库
	_, err := dm.db.Exec(`
		INSERT INTO download_tasks 
		(id, comic_id, title, type, cover, total_pages, downloaded_pages, 
		 current_ep, status, error, created_at, updated_at, description, extra, tags, author)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, task.ID, task.ComicID, task.Title, task.Type, task.Cover,
		task.TotalPages, task.DownloadedPages, task.CurrentEp,
		task.Status, task.Error, task.CreatedAt.Unix(), task.UpdatedAt.Unix(),
		task.Description, task.Extra, task.Tags, task.Author)

	if err != nil {
		return nil, err
	}

	dm.queue = append(dm.queue, task)

	// 如果没有正在下载的任务，自动开始
	if !dm.isDownloading {
		go dm.processQueue()
	}

	return task, nil
}

// GetDownloadQueue 获取下载队列
func (dm *DownloadManager) GetDownloadQueue() []*models.DownloadTask {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.queue
}

// Start 开始下载
func (dm *DownloadManager) Start() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.isDownloading {
		return fmt.Errorf("已经在下载中")
	}

	if len(dm.queue) == 0 {
		return fmt.Errorf("下载队列为空")
	}

	go dm.processQueue()
	return nil
}

// Pause 暂停下载
func (dm *DownloadManager) Pause() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.isDownloading {
		dm.stopChan <- true
		dm.isDownloading = false
		if dm.currentTask != nil {
			dm.currentTask.Status = "paused"
			dm.updateTaskStatus(dm.currentTask)
		}
	}
}

// IsDownloading 是否正在下载
func (dm *DownloadManager) IsDownloading() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.isDownloading
}

// CancelTask 取消任务
func (dm *DownloadManager) CancelTask(taskID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for i, task := range dm.queue {
		if task.ID == taskID {
			// 从队列中移除
			dm.queue = append(dm.queue[:i], dm.queue[i+1:]...)

			// 从数据库删除
			_, err := dm.db.Exec("DELETE FROM download_tasks WHERE id = ?", taskID)
			return err
		}
	}

	return fmt.Errorf("任务不存在")
}

// processQueue 处理下载队列
func (dm *DownloadManager) processQueue() {
	dm.mu.Lock()
	dm.isDownloading = true
	dm.mu.Unlock()

	defer func() {
		dm.mu.Lock()
		dm.isDownloading = false
		dm.currentTask = nil
		dm.mu.Unlock()
	}()

	for {
		dm.mu.Lock()
		if len(dm.queue) == 0 {
			dm.mu.Unlock()
			break
		}

		task := dm.queue[0]
		dm.currentTask = task
		dm.mu.Unlock()

		// 下载任务
		task.Status = "downloading"
		dm.updateTaskStatus(task)

		if err := dm.downloadTask(task); err != nil {
			task.Status = "error"
			task.Error = err.Error()
			dm.updateTaskStatus(task)
			fmt.Printf("下载失败: %s - %v\n", task.Title, err)
		} else {
			task.Status = "completed"
			dm.updateTaskStatus(task)
			fmt.Printf("下载完成: %s\n", task.Title)
		}

		// 从队列移除
		dm.mu.Lock()
		dm.queue = dm.queue[1:]
		dm.mu.Unlock()

		// 检查是否收到停止信号
		select {
		case <-dm.stopChan:
			return
		default:
		}
	}
}

// downloadTask 下载单个任务
func (dm *DownloadManager) downloadTask(task *models.DownloadTask) error {
	// 🆕 所有下载都使用直接下载模式（客户端拦截 URL）
	var extraCheck struct {
		DirectMode bool `json:"direct_mode"`
	}
	json.Unmarshal([]byte(task.Extra), &extraCheck)

	if !extraCheck.DirectMode {
		return fmt.Errorf("仅支持直接下载模式，请使用客户端拦截 URL 后发送到服务器")
	}

	fmt.Printf("[任务调度] 使用直接下载模式\n")
	return dm.downloadDirectComic(task)
}

// getImageHeaders 根据漫画类型获取图片下载请求头
func (dm *DownloadManager) getImageHeaders(comicType string, url string) map[string]string {
	headers := make(map[string]string)

	switch comicType {
	case "picacg":
		// Picacg 图片需要特定的请求头
		headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
		headers["Referer"] = "https://www.picacomic.com/"
		// Picacg图片通过其CDN提供，通常不需要特殊认证

	case "jm":
		// JM 图片请求头（更完整的浏览器请求头）
		headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
		headers["Referer"] = "https://18comic.vip/"
		headers["Accept"] = "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8"
		headers["Accept-Language"] = "zh-CN,zh;q=0.9,en;q=0.8"
		headers["Accept-Encoding"] = "gzip, deflate, br"
		headers["Sec-Fetch-Dest"] = "image"
		headers["Sec-Fetch-Mode"] = "no-cors"
		headers["Sec-Fetch-Site"] = "cross-site"

	case "ehentai":
		// EHentai 图片请求头
		headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
		headers["Referer"] = "https://e-hentai.org/"
		// EHentai图片服务器可能在 ehgt.org

	case "hitomi":
		// Hitomi 图片请求头
		headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
		headers["Referer"] = "https://hitomi.la/"

	case "nhentai":
		// NHentai 图片请求头
		headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
		headers["Referer"] = "https://nhentai.net/"

	case "htmanga", "htManga":
		// HTManga 图片请求头
		headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
		headers["Referer"] = "https://htmanga.com/"

	default:
		// 默认请求头
		headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}

	return headers
}

// downloadFileWithHeaders 下载文件（支持自定义请求头和重试）
func (dm *DownloadManager) downloadFileWithHeaders(url, filePath string, headers map[string]string) error {
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := dm.downloadFileOnce(url, filePath, headers)
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * time.Second
			fmt.Printf("[重试] 下载失败，%d秒后重试 (第 %d/%d 次): %v\n", waitTime/time.Second, attempt, maxRetries, err)
			time.Sleep(waitTime)
		}
	}

	return fmt.Errorf("下载失败（已重试 %d 次）: %w", maxRetries, lastErr)
}

// downloadFileOnce 下载文件（单次尝试）
func (dm *DownloadManager) downloadFileOnce(url, filePath string, headers map[string]string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// 设置默认请求头
	if headers == nil {
		headers = make(map[string]string)
	}
	if _, ok := headers["User-Agent"]; !ok {
		headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 设置超时
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("下载失败，状态码 %d: %s", resp.StatusCode, string(body))
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// parseTags 解析标签 JSON，返回所有标签和分类
func parseTags(tagsJSON string) (allTags []string, categories []string) {
	if tagsJSON == "" {
		return nil, nil
	}
	var tagsMap map[string][]string
	if err := json.Unmarshal([]byte(tagsJSON), &tagsMap); err != nil {
		return nil, nil
	}

	allTags = make([]string, 0)
	categories = make([]string, 0)

	for key, list := range tagsMap {
		// 提取分类（category/categories键）
		if key == "category" || key == "categories" {
			categories = append(categories, list...)
		}
		// 所有标签都加入到allTags
		allTags = append(allTags, list...)
	}

	return allTags, categories
}

// updateTaskStatus 更新任务状态
func (dm *DownloadManager) updateTaskStatus(task *models.DownloadTask) {
	task.UpdatedAt = time.Now()
	_, _ = dm.db.Exec(`
		UPDATE download_tasks 
		SET status = ?, error = ?, downloaded_pages = ?, current_ep = ?, total_pages = ?, updated_at = ?
		WHERE id = ?
	`, task.Status, task.Error, task.DownloadedPages, task.CurrentEp, task.TotalPages,
		task.UpdatedAt.Unix(), task.ID)
}

// generateComicID 为文件夹生成稳定的ID
func generateComicID(folderName string) string {
	// 使用MD5哈希生成稳定的ID
	hash := md5.Sum([]byte(folderName))
	return fmt.Sprintf("scanned_%x", hash[:8]) // 使用前8字节，足够唯一
}

// GetAllComics 获取所有已下载的漫画（包括扫描文件夹）
func (dm *DownloadManager) GetAllComics() ([]models.ComicDetail, error) {
	// 1. 从数据库读取已记录的漫画
	comicMap := make(map[string]*models.ComicDetail)

	rows, err := dm.db.Query(`
		SELECT id, title, author, description, cover, tags, categories,
		       eps_count, pages_count, type, time, size, directory, eps, downloaded_eps, detail_url
		FROM comics
		ORDER BY time DESC
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var comic models.ComicDetail
			var tagsJSON, categoriesJSON, epsJSON, downloadedEpsJSON string
			var timeUnix int64

			err := rows.Scan(
				&comic.ID, &comic.Title, &comic.Author, &comic.Description,
				&comic.Cover, &tagsJSON, &categoriesJSON, &comic.EpsCount,
				&comic.PagesCount, &comic.Type, &timeUnix, &comic.Size,
				&comic.Directory, &epsJSON, &downloadedEpsJSON, &comic.DetailURL,
			)
			if err == nil {
				// 确保即使JSON为空也初始化为空数组
				if tagsJSON != "" {
					json.Unmarshal([]byte(tagsJSON), &comic.Tags)
				}
				if comic.Tags == nil {
					comic.Tags = []string{}
				}

				if categoriesJSON != "" {
					json.Unmarshal([]byte(categoriesJSON), &comic.Categories)
				}
				if comic.Categories == nil {
					comic.Categories = []string{}
				}

				if epsJSON != "" {
					json.Unmarshal([]byte(epsJSON), &comic.Eps)
				}
				if comic.Eps == nil {
					comic.Eps = []string{}
				}

				if downloadedEpsJSON != "" {
					json.Unmarshal([]byte(downloadedEpsJSON), &comic.DownloadedEps)
				}
				if comic.DownloadedEps == nil {
					comic.DownloadedEps = []int{}
				}

				comic.Time = time.Unix(timeUnix, 0)
				comicMap[comic.Directory] = &comic
			}
		}
	}

	// 2. 扫描下载目录中的所有文件夹
	entries, err := os.ReadDir(dm.downloadPath)
	if err != nil {
		// 如果扫描失败，返回数据库中的漫画
		var comics []models.ComicDetail
		for _, comic := range comicMap {
			comics = append(comics, *comic)
		}
		return comics, nil
	}

	dirSet := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			dirName := entry.Name()
			dirSet[dirName] = true

			// 如果数据库中没有此文件夹的记录，创建一个基本记录
			if _, exists := comicMap[dirName]; !exists {
				info, _ := entry.Info()
				modTime := time.Now()
				if info != nil {
					modTime = info.ModTime()
				}

				// 扫描章节
				eps, downloadedEps := dm.scanComicFolder(filepath.Join(dm.downloadPath, dirName))

				// 为扫描的漫画生成稳定的ID
				scannedID := generateComicID(dirName)

				// 计算文件夹大小
				folderPath := filepath.Join(dm.downloadPath, dirName)
				folderSize := calculateFolderSize(folderPath)

				comicMap[dirName] = &models.ComicDetail{
					Comic: models.Comic{
						ID:          scannedID, // 使用哈希生成的稳定ID
						Title:       dirName,
						Author:      "未知",
						Description: "从文件夹扫描的漫画",
						Tags:        []string{},
						Categories:  []string{},
						Type:        "server", // 服务器漫画
						EpsCount:    len(eps),
						PagesCount:  0,          // 页数在阅读时动态获取
						Size:        folderSize, // 计算的实际大小（字节）
						Time:        modTime,
					},
					Directory:     dirName,
					Eps:           eps,
					DownloadedEps: downloadedEps,
				}
			}
		}
	}

	// 3. 转换为切片并返回
	var comics []models.ComicDetail
	for _, comic := range comicMap {
		comics = append(comics, *comic)
	}

	return comics, nil
}

// scanComicFolder 扫描漫画文件夹，获取章节信息
func (dm *DownloadManager) scanComicFolder(folderPath string) ([]string, []int) {
	var eps []string
	var downloadedEps []int

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return eps, downloadedEps
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// 尝试将文件夹名转换为数字（章节序号）
			if epNum, err := strconv.Atoi(entry.Name()); err == nil {
				eps = append(eps, fmt.Sprintf("第 %d 话", epNum))
				downloadedEps = append(downloadedEps, epNum)
			}
		}
	}

	// 排序
	sort.Ints(downloadedEps)

	return eps, downloadedEps
}

// GetComic 获取漫画详情
func (dm *DownloadManager) GetComic(id string) (*models.ComicDetail, error) {
	var comic models.ComicDetail
	var tagsJSON, categoriesJSON, epsJSON, downloadedEpsJSON string
	var timeUnix int64

	err := dm.db.QueryRow(`
		SELECT id, title, author, description, cover, tags, categories,
		       eps_count, pages_count, type, time, size, directory, eps, downloaded_eps, detail_url
		FROM comics WHERE id = ?
	`, id).Scan(
		&comic.ID, &comic.Title, &comic.Author, &comic.Description,
		&comic.Cover, &tagsJSON, &categoriesJSON, &comic.EpsCount,
		&comic.PagesCount, &comic.Type, &timeUnix, &comic.Size,
		&comic.Directory, &epsJSON, &downloadedEpsJSON, &comic.DetailURL,
	)

	if err != nil {
		// 如果数据库中找不到，尝试从文件系统扫描
		if err == sql.ErrNoRows {
			// 获取所有漫画（包括扫描的）
			allComics, scanErr := dm.GetAllComics()
			if scanErr != nil {
				return nil, err // 返回原始错误
			}

			// 查找匹配的漫画
			for _, c := range allComics {
				if c.ID == id {
					return &c, nil
				}
			}
		}
		return nil, err
	}

	// 确保即使JSON为空也初始化为空数组
	if tagsJSON != "" {
		json.Unmarshal([]byte(tagsJSON), &comic.Tags)
	}
	if comic.Tags == nil {
		comic.Tags = []string{}
	}

	if categoriesJSON != "" {
		json.Unmarshal([]byte(categoriesJSON), &comic.Categories)
	}
	if comic.Categories == nil {
		comic.Categories = []string{}
	}

	if epsJSON != "" {
		json.Unmarshal([]byte(epsJSON), &comic.Eps)
	}
	if comic.Eps == nil {
		comic.Eps = []string{}
	}

	if downloadedEpsJSON != "" {
		json.Unmarshal([]byte(downloadedEpsJSON), &comic.DownloadedEps)
	}
	if comic.DownloadedEps == nil {
		comic.DownloadedEps = []int{}
	}

	comic.Time = time.Unix(timeUnix, 0)

	return &comic, nil
}

// GetCoverPath 获取封面路径
func (dm *DownloadManager) GetCoverPath(id string) (string, error) {
	comic, err := dm.GetComic(id)
	if err != nil {
		return "", err
	}

	coverPath := filepath.Join(dm.downloadPath, comic.Directory, "cover.jpg")
	if _, err := os.Stat(coverPath); os.IsNotExist(err) {
		return "", fmt.Errorf("封面文件不存在")
	}

	return coverPath, nil
}

// GetEpisodePageCount 获取章节的页面数量
func (dm *DownloadManager) GetEpisodePageCount(id string, ep int) (int, error) {
	comic, err := dm.GetComic(id)
	if err != nil {
		return 0, err
	}

	var imagePath string
	if ep == 0 {
		// 没有章节的漫画
		imagePath = filepath.Join(dm.downloadPath, comic.Directory)
	} else {
		// 有章节的漫画
		imagePath = filepath.Join(dm.downloadPath, comic.Directory, fmt.Sprintf("%d", ep))
	}

	// 读取目录中的图片文件
	files, err := os.ReadDir(imagePath)
	if err != nil {
		return 0, err
	}

	// 统计图片文件数量（排除 cover.jpg）
	count := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		// 排除封面和非图片文件
		if name == "cover.jpg" || name == "cover.png" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			count++
		}
	}

	return count, nil
}

// GetImagePath 获取图片路径
func (dm *DownloadManager) GetImagePath(id string, ep int, page int) (string, error) {
	comic, err := dm.GetComic(id)
	if err != nil {
		return "", err
	}

	var imagePath string
	if ep == 0 {
		// 没有章节的漫画
		imagePath = filepath.Join(dm.downloadPath, comic.Directory)
	} else {
		// 有章节的漫画
		imagePath = filepath.Join(dm.downloadPath, comic.Directory, fmt.Sprintf("%d", ep))
	}

	// 查找匹配的文件
	files, err := os.ReadDir(imagePath)
	if err != nil {
		return "", err
	}

	// 尝试多种页面编号格式
	pageFormats := []string{
		fmt.Sprintf("%03d", page), // 001, 002, ...
		fmt.Sprintf("%d", page),   // 1, 2, ...
		fmt.Sprintf("%02d", page), // 01, 02, ...
	}

	for _, file := range files {
		name := file.Name()
		nameWithoutExt := strings.TrimSuffix(name, filepath.Ext(name))

		for _, pageStr := range pageFormats {
			if nameWithoutExt == pageStr {
				return filepath.Join(imagePath, name), nil
			}
		}
	}

	return "", fmt.Errorf("图片文件不存在: page=%d", page)
}

// DeleteComic 删除漫画
func (dm *DownloadManager) DeleteComic(id string) error {
	comic, err := dm.GetComic(id)
	if err != nil {
		return err
	}

	// 删除文件
	comicPath := filepath.Join(dm.downloadPath, comic.Directory)
	if err := os.RemoveAll(comicPath); err != nil {
		return err
	}

	// 从数据库删除
	_, err = dm.db.Exec("DELETE FROM comics WHERE id = ?", id)
	return err
}

// downloadFile 下载文件
func downloadFile(url string, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

type TaskRepository struct {
	db           *sql.DB
	downloadPath string
}

func NewTaskRepository(db *sql.DB, downloadPath string) *TaskRepository {
	return &TaskRepository{db: db, downloadPath: downloadPath}
}

func (repo *TaskRepository) UpdateProgress(task *models.DownloadTask, downloadedPages int, currentEp int) error {
	task.DownloadedPages = downloadedPages
	task.CurrentEp = currentEp
	task.UpdatedAt = time.Now()
	_, err := repo.db.Exec(`
		UPDATE download_tasks 
		SET downloaded_pages = ?, current_ep = ?, updated_at = ?, status = ?
		WHERE id = ?
	`, task.DownloadedPages, task.CurrentEp, task.UpdatedAt.Unix(), task.Status, task.ID)
	return err
}

func (repo *TaskRepository) SaveComicDetail(comic *models.ComicDetail) error {
	tagsJSON, _ := json.Marshal(comic.Tags)
	categoriesJSON, _ := json.Marshal(comic.Categories)
	epsJSON, _ := json.Marshal(comic.Eps)
	downloadedEpsJSON, _ := json.Marshal(comic.DownloadedEps)

	_, err := repo.db.Exec(`
		INSERT INTO comics (
			id, title, author, description, cover, tags, categories,
			eps_count, pages_count, type, time, size, directory, eps, downloaded_eps, detail_url
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			author = excluded.author,
			description = excluded.description,
			cover = excluded.cover,
			tags = excluded.tags,
			categories = excluded.categories,
			eps_count = excluded.eps_count,
			pages_count = excluded.pages_count,
			type = excluded.type,
			time = excluded.time,
			size = excluded.size,
			detail_url = excluded.detail_url,
			directory = excluded.directory,
			eps = excluded.eps,
			downloaded_eps = excluded.downloaded_eps
	`,
		comic.ID,
		comic.Title,
		comic.Author,
		comic.Description,
		comic.Cover,
		string(tagsJSON),
		string(categoriesJSON),
		comic.EpsCount,
		comic.PagesCount,
		comic.Type,
		comic.Time.Unix(),
		comic.Size,
		comic.Directory,
		string(epsJSON),
		string(downloadedEpsJSON),
		comic.DetailURL,
	)
	return err
}

func (repo *TaskRepository) SaveFile(path string, reader io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func (repo *TaskRepository) SaveBytes(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (repo *TaskRepository) AppendDownloadedEp(comicID string, ep int) error {
	var downloaded string
	err := repo.db.QueryRow("SELECT downloaded_eps FROM comics WHERE id = ?", comicID).Scan(&downloaded)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var eps []int
	if downloaded != "" {
		if err := json.Unmarshal([]byte(downloaded), &eps); err != nil {
			return err
		}
	}

	for _, existing := range eps {
		if existing == ep {
			return nil
		}
	}

	eps = append(eps, ep)
	data, _ := json.Marshal(eps)
	_, err = repo.db.Exec("UPDATE comics SET downloaded_eps = ? WHERE id = ?", string(data), comicID)
	return err
}

// SubmitDirectDownload 提交直接下载任务（方案2：客户端已获取URL）
func (dm *DownloadManager) SubmitDirectDownload(reqData interface{}) (string, error) {
	// 因为不能直接导入 handlers 包（会循环依赖），所以用反射处理
	data, err := json.Marshal(reqData)
	if err != nil {
		return "", err
	}

	var req struct {
		ComicID     string              `json:"comic_id"`
		Type        string              `json:"type"`
		Title       string              `json:"title"`
		Cover       string              `json:"cover"`
		Author      string              `json:"author"`
		Description string              `json:"description"`
		DetailURL   string              `json:"detail_url"` // 详情页链接
		Tags        map[string][]string `json:"tags"`
		Episodes    []struct {
			Order    int               `json:"order"`
			Name     string            `json:"name"`
			PageURLs []string          `json:"page_urls"`
			Headers  map[string]string `json:"headers"` // 客户端提供的 HTTP headers
		} `json:"episodes"`
	}

	if err := json.Unmarshal(data, &req); err != nil {
		return "", err
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 检查是否已存在相同的下载任务（队列中）
	for _, existingTask := range dm.queue {
		if existingTask.ComicID == req.ComicID &&
			(existingTask.Status == "pending" || existingTask.Status == "downloading" || existingTask.Status == "paused") {
			log.Printf("[DownloadManager] 漫画 %s 已在下载队列中，任务ID: %s，状态: %s",
				req.ComicID, existingTask.ID, existingTask.Status)
			return existingTask.ID, nil
		}
	}

	// 检查数据库中是否有未完成的任务
	var existingTaskID string
	err = dm.db.QueryRow(`
		SELECT id FROM download_tasks 
		WHERE comic_id = ? AND status IN ('pending', 'downloading', 'paused')
		LIMIT 1
	`, req.ComicID).Scan(&existingTaskID)

	if err == nil {
		// 找到了现有任务
		log.Printf("[DownloadManager] 数据库中已有漫画 %s 的下载任务: %s", req.ComicID, existingTaskID)
		return existingTaskID, nil
	}

	// 创建任务
	taskID := fmt.Sprintf("direct_%d", time.Now().UnixNano())

	title := req.Title
	if title == "" {
		title = "未命名漫画"
	}

	// 计算总页数
	totalPages := 0
	for _, ep := range req.Episodes {
		totalPages += len(ep.PageURLs)
	}

	task := &models.DownloadTask{
		ID:              taskID,
		ComicID:         req.ComicID,
		Type:            req.Type,
		Title:           title,
		Cover:           req.Cover,
		Description:     req.Description,
		Author:          req.Author,
		Status:          "pending",
		TotalPages:      totalPages,
		DownloadedPages: 0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// 将 episodes 数据和 detail_url 存入 Extra
	extraData := map[string]interface{}{
		"direct_mode": true,
		"episodes":    req.Episodes,
		"detail_url":  req.DetailURL,
	}
	extraJSON, _ := json.Marshal(extraData)
	task.Extra = string(extraJSON)

	// 保存到数据库
	tagsJSON, _ := json.Marshal(req.Tags)
	task.Tags = string(tagsJSON)

	_, err = dm.db.Exec(`
		INSERT INTO download_tasks 
		(id, comic_id, type, title, status, cover, description, tags, author, extra, downloaded_pages, total_pages, current_ep, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.ID, task.ComicID, task.Type, task.Title, task.Status,
		task.Cover, task.Description, task.Tags, task.Author,
		task.Extra, task.DownloadedPages, task.TotalPages, task.CurrentEp,
		task.CreatedAt.Unix(), task.UpdatedAt.Unix())

	if err != nil {
		return "", err
	}

	log.Printf("[DownloadManager] 创建新下载任务: %s, 漫画ID: %s, 标题: %s", taskID, req.ComicID, title)

	// 加入下载队列
	dm.queue = append(dm.queue, task)

	go dm.processQueue()

	return taskID, nil
}

// sanitizeFolderName 将字符串转换为安全的文件夹名称
// 使用与客户端相同的规则
func sanitizeFolderName(name string) string {
	// 1. 替换非法字符为空格（与客户端一致）
	replacer := strings.NewReplacer(
		"<", " ",
		">", " ",
		":", " ",
		"\"", " ",
		"/", " ",
		"\\", " ",
		"|", " ",
		"?", " ",
		"*", " ",
	)
	safe := replacer.Replace(name)

	// 2. 去除首尾空格
	safe = strings.TrimSpace(safe)

	// 3. 限制长度为 255 字节（与客户端一致）
	// 逐字符删除末尾，避免破坏 UTF-8 编码
	for len(safe) > 0 && len([]byte(safe)) > 255 {
		runes := []rune(safe)
		safe = string(runes[:len(runes)-1])
	}

	// 如果为空，使用默认名称
	if safe == "" {
		safe = "untitled"
	}

	return safe
}

// extractBookIdFromUrl 从 JM 图片 URL 中提取 bookId
// 与客户端逻辑一致：
// 1. url.substring(i + 1, url.length - 5)
// 2. bookId.replaceAll(RegExp(r"\..+"), "")
func extractBookIdFromUrl(url string) string {
	// JM URL 格式: https://cdn.../photos/12345/abc123.webp
	// 客户端逻辑：从最后一个 / 之后开始，去掉最后5个字符（.webp）
	
	// 先移除查询参数
	url = strings.Split(url, "?")[0]
	
	// 找到最后一个 /
	lastSlash := strings.LastIndex(url, "/")
	if lastSlash == -1 || lastSlash == len(url)-1 {
		return ""
	}
	
	// 提取文件名部分
	filename := url[lastSlash+1:]
	
	// 去掉最后5个字符（与客户端一致）
	if len(filename) > 5 {
		filename = filename[:len(filename)-5]
	}
	
	// 移除第一个 . 之后的所有内容（与客户端一致）
	if dotIndex := strings.Index(filename, "."); dotIndex != -1 {
		filename = filename[:dotIndex]
	}
	
	return filename
}

// downloadDirectComic 直接下载模式（客户端已获取URL）
func (dm *DownloadManager) downloadDirectComic(task *models.DownloadTask) error {
	// 解析 Extra 中的 episodes 数据和 detail_url
	var extra struct {
		DirectMode bool   `json:"direct_mode"`
		DetailURL  string `json:"detail_url"` // 详情页链接
		Episodes   []struct {
			Order            int               `json:"order"`
			Name             string            `json:"name"`
			PageURLs         []string          `json:"page_urls"`
			Headers          map[string]string `json:"headers"`           // 客户端提供的 HTTP headers
			DescrambleParams map[string]string `json:"descramble_params"` // 反混淆参数（可选）
		} `json:"episodes"`
	}

	if err := json.Unmarshal([]byte(task.Extra), &extra); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	if !extra.DirectMode {
		return fmt.Errorf("不是直接下载模式")
	}

	repo := NewTaskRepository(dm.db, dm.downloadPath)

	// 使用漫画标题作为文件夹名（安全化处理）
	baseFolderName := sanitizeFolderName(task.Title)
	folderName := baseFolderName

	// 检查文件夹是否存在，如果存在则添加数字后缀
	counter := 1
	for {
		testPath := filepath.Join(dm.downloadPath, folderName)
		if _, err := os.Stat(testPath); os.IsNotExist(err) {
			// 文件夹不存在，可以使用
			break
		}
		// 文件夹已存在，尝试下一个名字
		counter++
		folderName = fmt.Sprintf("%s_%d", baseFolderName, counter)
	}

	downloadDir := filepath.Join(dm.downloadPath, folderName)
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return fmt.Errorf("创建下载目录失败: %w", err)
	}

	// 获取该漫画类型的图片请求头
	imageHeaders := dm.getImageHeaders(task.Type, "")

	// 下载封面
	if task.Cover != "" {
		fmt.Printf("[直接下载] 下载封面: %s\n", task.Cover)
		coverPath := filepath.Join(downloadDir, "cover.jpg")
		if err := dm.downloadFileWithHeaders(task.Cover, coverPath, imageHeaders); err != nil {
			fmt.Printf("[警告] 封面下载失败: %v\n", err)
			// 封面下载失败不阻止整个任务
		} else {
			fmt.Printf("[直接下载] ✅ 封面下载成功\n")
		}
	}

	downloadedPages := 0

	fmt.Printf("[直接下载] 开始下载 %d 个章节...\n", len(extra.Episodes))
	for _, ep := range extra.Episodes {
		epDir := filepath.Join(downloadDir, fmt.Sprintf("%d", ep.Order))
		if err := os.MkdirAll(epDir, 0755); err != nil {
			return fmt.Errorf("创建章节目录失败: %w", err)
		}

		fmt.Printf("[直接下载] 章节 %d (%s) 共有 %d 张图片\n", ep.Order, ep.Name, len(ep.PageURLs))

		// 优先使用客户端提供的 headers，否则使用服务器端默认的
		episodeHeaders := imageHeaders
		if ep.Headers != nil && len(ep.Headers) > 0 {
			episodeHeaders = ep.Headers
			fmt.Printf("[直接下载] 使用客户端提供的 headers: %d 个\n", len(episodeHeaders))
		} else {
			fmt.Printf("[直接下载] 使用服务器端默认 headers\n")
		}

		for index, pageURL := range ep.PageURLs {
			filePath := filepath.Join(epDir, fmt.Sprintf("%03d.jpg", index+1))
			fmt.Printf("[直接下载] 正在下载章节 %d 第 %d/%d 页\n", ep.Order, index+1, len(ep.PageURLs))

			// 使用章节对应的请求头下载图片
			if err := dm.downloadFileWithHeaders(pageURL, filePath, episodeHeaders); err != nil {
				fmt.Printf("[错误] 下载失败: %v\n", err)
				return fmt.Errorf("下载章节 %d 第 %d 页失败: %w", ep.Order, index+1, err)
			}

			// 验证文件大小
			info, _ := os.Stat(filePath)
			if info.Size() < 100 {
				return fmt.Errorf("下载的文件过小（可能失败）: %d bytes", info.Size())
			}

			// 稍微等待确保文件完全写入磁盘
			time.Sleep(10 * time.Millisecond)

			// 如果有反混淆参数，进行反混淆处理
			if ep.DescrambleParams != nil && len(ep.DescrambleParams) > 0 {
				epsId := ep.DescrambleParams["epsId"]
				scrambleId := ep.DescrambleParams["scrambleId"]

				// 从 URL 中提取 bookId
				bookId := extractBookIdFromUrl(pageURL)

				fmt.Printf("[反混淆] 处理图片: epsId=%s, scrambleId=%s, bookId=%s\n", epsId, scrambleId, bookId)
				if err := DescrambleJmImage(filePath, epsId, scrambleId, bookId); err != nil {
					fmt.Printf("[警告] 图片反混淆失败: %v\n", err)
					// 不中断下载，继续处理其他图片
				} else {
					fmt.Printf("[反混淆] ✅ 图片反混淆成功\n")
				}
			}

			downloadedPages++
			task.DownloadedPages = downloadedPages
			task.CurrentEp = ep.Order
			dm.updateTaskStatus(task)
		}

		// 保存章节信息
		if err := repo.AppendDownloadedEp(task.ComicID, ep.Order); err != nil {
			fmt.Printf("[警告] 保存章节信息失败: %v\n", err)
		}
	}

	// 保存漫画详情
	// 解析标签和分类
	allTags, categories := parseTags(task.Tags)

	var epNames []string
	var epOrders []int
	for _, ep := range extra.Episodes {
		epNames = append(epNames, ep.Name)
		epOrders = append(epOrders, ep.Order)
	}

	// 计算漫画文件夹大小
	folderSize := calculateFolderSize(downloadDir)

	detail := &models.ComicDetail{
		Comic: models.Comic{
			ID:          task.ComicID,
			Title:       task.Title,
			Cover:       task.Cover,
			Author:      task.Author,
			Description: task.Description,
			Tags:        allTags,    // 所有标签
			Categories:  categories, // 分类（从tags中提取的category键）
			Type:        task.Type,
			EpsCount:    len(extra.Episodes),
			PagesCount:  task.TotalPages,
			Size:        folderSize, // 计算的实际大小（字节）
			Time:        time.Now(),
			DetailURL:   extra.DetailURL, // 详情页链接
		},
		Directory:     folderName, // 使用安全的文件夹名称
		Eps:           epNames,
		DownloadedEps: epOrders,
	}

	if err := repo.SaveComicDetail(detail); err != nil {
		return fmt.Errorf("保存漫画详情失败: %w", err)
	}

	fmt.Printf("[直接下载] ✅ 下载完成！\n")
	return nil
}

// ImportComicFromClient 从客户端导入已下载的漫画
func (dm *DownloadManager) ImportComicFromClient(r *http.Request, comicID, title, comicType, author, description, coverURL string) error {
	fmt.Printf("[导入] 开始导入漫画: %s\n", title)

	repo := NewTaskRepository(dm.db, dm.downloadPath)

	// 1. 创建漫画目录
	folderName := sanitizeFolderName(title)
	counter := 1
	for {
		testPath := filepath.Join(dm.downloadPath, folderName)
		if _, err := os.Stat(testPath); os.IsNotExist(err) {
			break
		}
		counter++
		folderName = fmt.Sprintf("%s_%d", sanitizeFolderName(title), counter)
	}

	comicDir := filepath.Join(dm.downloadPath, folderName)
	if err := os.MkdirAll(comicDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 2. 获取上传的文件
	form := r.MultipartForm
	if form == nil {
		return fmt.Errorf("没有上传文件")
	}

	files := form.File["files"]
	if len(files) == 0 {
		return fmt.Errorf("没有上传文件")
	}

	// 3. 解析章节结构并保存文件
	// 客户端上传格式: ep1_page001.jpg, ep1_page002.jpg, ep2_page001.jpg, ...
	// 或 cover.jpg
	episodeMap := make(map[int][]string) // ep -> 页面文件列表
	var epsCount, totalPages int

	fmt.Printf("[导入] 开始处理 %d 个上传文件...\n", len(files))

	for _, fileHeader := range files {
		// 打开上传的文件
		file, err := fileHeader.Open()
		if err != nil {
			fmt.Printf("[警告] 打开文件失败: %v\n", err)
			continue
		}

		filename := fileHeader.Filename
		fmt.Printf("[导入] 处理文件: %s\n", filename)

		// 保存封面
		if filename == "cover.jpg" || filename == "cover.png" {
			ext := filepath.Ext(filename)
			coverPath := filepath.Join(comicDir, "cover"+ext)
			if err := saveUploadedFile(file, coverPath); err != nil {
				fmt.Printf("[警告] 保存封面失败: %v\n", err)
			} else {
				fmt.Printf("[导入] ✅ 封面已保存: %s\n", coverPath)
			}
			file.Close()
			continue
		}

		// 解析章节和页面信息 (格式: ep1_page001.jpg 或 ep1_page001.png)
		// 使用正则表达式来匹配不同的扩展名
		var epNum, pageNum int
		var ext string

		// 尝试解析文件名
		matched := false
		for _, pattern := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif"} {
			testName := strings.TrimSuffix(filename, pattern)
			n, err := fmt.Sscanf(testName, "ep%d_page%03d", &epNum, &pageNum)
			if err == nil && n == 2 {
				ext = pattern
				matched = true
				break
			}
		}

		if !matched {
			fmt.Printf("[警告] 文件名格式不正确: %s (无法解析章节和页面信息)\n", filename)
			file.Close()
			continue
		}

		fmt.Printf("[导入] 解析成功: 章节 %d, 页面 %d, 扩展名 %s\n", epNum, pageNum, ext)

		// 创建章节目录
		epDir := filepath.Join(comicDir, fmt.Sprintf("%d", epNum))
		if err := os.MkdirAll(epDir, 0755); err != nil {
			file.Close()
			return fmt.Errorf("创建章节目录失败: %w", err)
		}

		// 保存图片，保留原始扩展名
		imagePath := filepath.Join(epDir, fmt.Sprintf("%03d%s", pageNum, ext))
		if err := saveUploadedFile(file, imagePath); err != nil {
			file.Close()
			return fmt.Errorf("保存图片失败: %w", err)
		}
		fmt.Printf("[导入] ✅ 图片已保存: %s\n", imagePath)
		file.Close()

		// 记录章节信息
		if _, exists := episodeMap[epNum]; !exists {
			episodeMap[epNum] = []string{}
			epsCount++
		}
		episodeMap[epNum] = append(episodeMap[epNum], imagePath)
		totalPages++
	}

	fmt.Printf("[导入] 文件处理完成: %d 个章节, %d 页\n", epsCount, totalPages)

	// 4. 构建章节列表
	var epOrders []int
	for ep := range episodeMap {
		epOrders = append(epOrders, ep)
	}
	sort.Ints(epOrders)

	// 尝试从客户端获取章节名称列表
	var epNames []string
	epsJSON := r.PostFormValue("eps")
	if epsJSON != "" {
		if err := json.Unmarshal([]byte(epsJSON), &epNames); err != nil {
			fmt.Printf("[警告] 解析章节名称失败: %v\n", err)
			// 如果解析失败，使用默认名称
			for _, ep := range epOrders {
				epNames = append(epNames, fmt.Sprintf("第 %d 话", ep))
			}
		}
	} else {
		// 没有提供章节名称，使用默认名称
		for _, ep := range epOrders {
			epNames = append(epNames, fmt.Sprintf("第 %d 话", ep))
		}
	}

	// 5. 解析标签和分类
	tagsJSON := r.PostFormValue("tags")
	allTags, categories := parseTags(tagsJSON)

	// 6. 获取下载时间
	var downloadTime time.Time
	downloadTimeStr := r.PostFormValue("download_time")
	if downloadTimeStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, downloadTimeStr)
		if err == nil {
			downloadTime = parsedTime
		} else {
			downloadTime = time.Now()
		}
	} else {
		downloadTime = time.Now()
	}

	// 7. 保存到数据库
	// 计算漫画文件夹大小
	folderSize := calculateFolderSize(comicDir)

	detail := &models.ComicDetail{
		Comic: models.Comic{
			ID:          comicID,
			Title:       title,
			Cover:       coverURL,
			Author:      author,
			Description: description,
			Tags:        allTags,    // 所有标签
			Categories:  categories, // 分类（从tags中提取的category键）
			Type:        comicType,
			EpsCount:    epsCount,
			PagesCount:  totalPages,
			Size:        folderSize,   // 计算的实际大小（字节）
			Time:        downloadTime, // 使用客户端提供的下载时间
		},
		Directory:     folderName,
		Eps:           epNames,
		DownloadedEps: epOrders,
	}

	if err := repo.SaveComicDetail(detail); err != nil {
		return fmt.Errorf("保存漫画详情失败: %w", err)
	}

	fmt.Printf("[导入] ✅ 导入完成！共 %d 个章节，%d 页，大小 %.2f MB\n", epsCount, totalPages, float64(folderSize)/(1024*1024))
	return nil
}

// saveUploadedFile 保存上传的文件
func saveUploadedFile(src multipart.File, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// calculateFolderSize 计算文件夹大小（字节）
func calculateFolderSize(path string) int64 {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		fmt.Printf("[警告] 计算文件夹大小失败: %v\n", err)
		return 0
	}
	return size
}
