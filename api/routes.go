package api

import (
	"pica-comic-server/api/handlers"

	"github.com/gin-gonic/gin"
)

// SetupRoutes 注册所有路由
func SetupRoutes(r *gin.Engine) {
	// 启用 CORS
	r.Use(corsMiddleware())

	api := r.Group("/api")
	{
		// 漫画管理（无需账号）
		comics := api.Group("/comics")
		{
			comics.GET("", handlers.GetComics)
			comics.GET("/:id", handlers.GetComicDetail)
			comics.GET("/:id/cover", handlers.GetComicCover)
			comics.GET("/:id/:ep/info", handlers.GetEpisodeInfo) // 获取章节页面数量
			comics.GET("/:id/:ep/:page", handlers.GetComicPage)
			comics.DELETE("/:id", handlers.DeleteComic)
		}

		// 下载管理
		download := api.Group("/download")
		{
			download.POST("", handlers.AddDownloadTask)
			download.POST("/direct", handlers.SubmitDirectDownload) // 方案2：直接下载
			download.POST("/import", handlers.ImportComic)          // 导入客户端已下载的漫画
			download.GET("/queue", handlers.GetDownloadQueue)
			download.POST("/start", handlers.StartDownload)
			download.POST("/pause", handlers.PauseDownload)
			download.DELETE("/:id", handlers.CancelDownload)
		}

		// PicaComic API
		picacg := api.Group("/picacg")
		{
			picacg.POST("/login", handlers.PicacgLogin)
			picacg.GET("/categories", handlers.PicacgGetCategories)
			picacg.GET("/search", handlers.PicacgSearch)
			picacg.GET("/comic/:id", handlers.PicacgGetComic)
			picacg.GET("/comic/:id/eps", handlers.PicacgGetEps)
		}
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

// CORS 中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
