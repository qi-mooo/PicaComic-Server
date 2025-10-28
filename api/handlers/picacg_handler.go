package handlers

import (
	"net/http"
	"strconv"

	"pica-comic-server/models"
	"pica-comic-server/picacg"

	"github.com/gin-gonic/gin"
)

var client *picacg.Client

// PicacgLogin PicaComic 登录
func PicacgLogin(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
		})
		return
	}

	client = picacg.NewClient()
	token, err := client.Login(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "登录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   token,
	})
}

// PicacgGetCategories 获取分类
func PicacgGetCategories(c *gin.Context) {
	if client == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}

	categories, err := client.GetCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
	})
}

// PicacgSearch 搜索漫画
func PicacgSearch(c *gin.Context) {
	if client == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}

	keyword := c.Query("keyword")
	sort := c.DefaultQuery("sort", "dd")
	pageStr := c.DefaultQuery("page", "1")

	page, err := strconv.Atoi(pageStr)
	if err != nil {
		page = 1
	}

	result, err := client.Search(keyword, sort, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// PicacgGetComic 获取漫画信息
func PicacgGetComic(c *gin.Context) {
	if client == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}

	id := c.Param("id")

	comic, err := client.GetComicInfo(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, comic)
}

// PicacgGetEps 获取漫画章节
func PicacgGetEps(c *gin.Context) {
	if client == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}

	id := c.Param("id")

	eps, err := client.GetEps(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"eps": eps,
	})
}
