package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"pica-comic-server/api"
	"pica-comic-server/services"

	"github.com/gin-gonic/gin"
)

func main() {
	// 解析命令行参数
	port := flag.String("port", "8080", "服务器端口")
	host := flag.String("host", "0.0.0.0", "服务器地址")
	downloadPath := flag.String("download-path", "", "下载目录路径")
	flag.Parse()

	fmt.Println("=================================")
	fmt.Println("PicaComic 服务器")
	fmt.Println("=================================")
	fmt.Println()

	// 初始化服务
	if err := initServices(*downloadPath); err != nil {
		log.Fatalf("初始化服务失败: %v", err)
	}

	// 创建 Gin 路由
	r := gin.Default()

	// 注册路由
	api.SetupRoutes(r)

	// 打印 API 信息
	printAPIInfo(*host, *port)

	// 启动服务器
	addr := fmt.Sprintf("%s:%s", *host, *port)
	fmt.Printf("\n服务器启动成功！正在监听 %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

func initServices(downloadPath string) error {
	fmt.Println("正在初始化服务...")

	// 设置数据目录
	dataDir := filepath.Join(".", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 设置下载目录
	if downloadPath == "" {
		downloadPath = filepath.Join(dataDir, "download")
	}
	if err := os.MkdirAll(downloadPath, 0755); err != nil {
		return fmt.Errorf("创建下载目录失败: %w", err)
	}

	// 初始化下载管理器
	if err := services.InitDownloadManager(downloadPath); err != nil {
		return fmt.Errorf("初始化下载管理器失败: %w", err)
	}

	fmt.Println("服务初始化完成！")
	fmt.Printf("数据目录: %s\n", dataDir)
	fmt.Printf("下载目录: %s\n", downloadPath)
	fmt.Println()

	return nil
}

func printAPIInfo(host, port string) {
	fmt.Println("\n可用的 API 端点:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("漫画管理:")
	fmt.Println("  GET    /api/comics              - 获取所有已下载的漫画")
	fmt.Println("  GET    /api/comics/:id          - 获取漫画详情")
	fmt.Println("  GET    /api/comics/:id/cover    - 获取漫画封面")
	fmt.Println("  GET    /api/comics/:id/:ep/:page - 获取漫画页面图片")
	fmt.Println("  DELETE /api/comics/:id          - 删除漫画")
	fmt.Println()
	fmt.Println("下载管理:")
	fmt.Println("  POST   /api/download            - 添加下载任务")
	fmt.Println("  GET    /api/download/queue      - 获取下载队列")
	fmt.Println("  POST   /api/download/start      - 开始/继续下载")
	fmt.Println("  POST   /api/download/pause      - 暂停下载")
	fmt.Println("  DELETE /api/download/:id        - 取消下载任务")
	fmt.Println()
	fmt.Println("PicaComic API:")
	fmt.Println("  POST   /api/picacg/login        - 登录 PicaComic")
	fmt.Println("  GET    /api/picacg/categories   - 获取分类")
	fmt.Println("  GET    /api/picacg/search       - 搜索漫画")
	fmt.Println("  GET    /api/picacg/comic/:id    - 获取漫画信息")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
