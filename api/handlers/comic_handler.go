package handlers

import (
	"net/http"
	"strconv"

	"pica-comic-server/services"

	"github.com/gin-gonic/gin"
)

// GetComics 获取所有已下载的漫画
func GetComics(c *gin.Context) {
	comics, err := services.GetDownloadManager().GetAllComics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"comics": comics,
		"total":  len(comics),
	})
}

// GetComicDetail 获取漫画详情
func GetComicDetail(c *gin.Context) {
	id := c.Param("id")

	comic, err := services.GetDownloadManager().GetComic(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "漫画不存在",
		})
		return
	}

	c.JSON(http.StatusOK, comic)
}

// GetComicCover 获取漫画封面
func GetComicCover(c *gin.Context) {
	id := c.Param("id")

	coverPath, err := services.GetDownloadManager().GetCoverPath(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "封面不存在",
		})
		return
	}

	c.File(coverPath)
}

// GetEpisodeInfo 获取章节信息（页面数量）
func GetEpisodeInfo(c *gin.Context) {
	id := c.Param("id")
	epStr := c.Param("ep")

	ep, err := strconv.Atoi(epStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的章节号",
		})
		return
	}

	pageCount, err := services.GetDownloadManager().GetEpisodePageCount(id, ep)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "章节不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"episode":    ep,
		"page_count": pageCount,
	})
}

// GetComicPage 获取漫画页面
func GetComicPage(c *gin.Context) {
	id := c.Param("id")
	epStr := c.Param("ep")
	pageStr := c.Param("page")

	ep, err := strconv.Atoi(epStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的章节号",
		})
		return
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的页码",
		})
		return
	}

	imagePath, err := services.GetDownloadManager().GetImagePath(id, ep, page)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "图片不存在",
		})
		return
	}

	// 设置缓存头
	c.Header("Cache-Control", "public, max-age=31536000")
	c.File(imagePath)
}

// DeleteComic 删除漫画
func DeleteComic(c *gin.Context) {
	id := c.Param("id")

	if err := services.GetDownloadManager().DeleteComic(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "删除成功",
	})
}
