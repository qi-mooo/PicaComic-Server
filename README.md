# PicaComic 服务器

基于 Go 语言的 PicaComic 漫画下载和管理服务器。

## 功能特性

- 🚀 支持 PicaComic 漫画下载
- 📦 漫画存储和管理
- 🌐 RESTful API 接口
- 📱 客户端可远程访问
- 💾 SQLite 数据库存储
- ⚡ 并发下载支持
- 🔄 下载队列管理

## 快速开始

### 环境要求

- Go 1.21 或更高版本
- SQLite3

### 安装依赖

```bash
cd server
go mod download
```

### 编译

```bash
go build -o pica-server main.go
```

### 运行

```bash
# 使用默认配置运行
./pica-server

# 指定端口和下载目录
./pica-server -port 8080 -download-path /path/to/downloads

# 指定监听地址
./pica-server -host 0.0.0.0 -port 8080
```

### 命令行参数

- `-port` 或 `-p`: 服务器端口（默认: 8080）
- `-host` 或 `-h`: 监听地址（默认: 0.0.0.0）
- `-download-path` 或 `-d`: 下载目录路径（默认: ./data/download）

## API 文档

### 漫画管理

#### 获取所有已下载的漫画

```http
GET /api/comics
```

响应示例：

```json
{
  "comics": [
    {
      "id": "comic-id",
      "title": "漫画标题",
      "author": "作者",
      "cover": "封面URL",
      "tags": ["标签1", "标签2"],
      "eps_count": 10,
      "size": 1024000
    }
  ],
  "total": 1
}
```

#### 获取漫画详情

```http
GET /api/comics/:id
```

#### 获取漫画封面

```http
GET /api/comics/:id/cover
```

返回图片文件。

#### 获取漫画页面

```http
GET /api/comics/:id/:ep/:page
```

参数：
- `id`: 漫画ID
- `ep`: 章节号（从1开始，无章节的漫画使用0）
- `page`: 页码（从0开始）

返回图片文件。

#### 删除漫画

```http
DELETE /api/comics/:id
```

### 下载管理

#### 添加下载任务

```http
POST /api/download
Content-Type: application/json

{
  "type": "picacg",
  "comic_id": "漫画ID",
  "eps": [1, 2, 3]  // 可选，不指定则下载全部章节
}
```

#### 获取下载队列

```http
GET /api/download/queue
```

响应示例：

```json
{
  "queue": [
    {
      "id": "task-id",
      "comic_id": "comic-id",
      "title": "漫画标题",
      "status": "downloading",
      "total_pages": 100,
      "downloaded_pages": 50
    }
  ],
  "total": 1,
  "is_downloading": true
}
```

#### 开始/继续下载

```http
POST /api/download/start
```

#### 暂停下载

```http
POST /api/download/pause
```

#### 取消下载任务

```http
DELETE /api/download/:id
```

### PicaComic API

#### 登录

```http
POST /api/picacg/login
Content-Type: application/json

{
  "email": "your-email@example.com",
  "password": "your-password"
}
```

#### 获取分类

```http
GET /api/picacg/categories
```

需要先登录。

#### 搜索漫画

```http
GET /api/picacg/search?keyword=关键词&sort=dd&page=1
```

参数：
- `keyword`: 搜索关键词
- `sort`: 排序方式（dd=最新, da=最旧, ld=最多爱心, vd=最多观看）
- `page`: 页码

需要先登录。

#### 获取漫画信息

```http
GET /api/picacg/comic/:id
```

需要先登录。

#### 获取漫画章节

```http
GET /api/picacg/comic/:id/eps
```

需要先登录。

## 数据存储

### 目录结构

```
data/
├── download/           # 下载目录
│   ├── download.db    # SQLite 数据库
│   └── [comic-name]/  # 漫画目录
│       ├── cover.jpg  # 封面
│       ├── 1/         # 第1章
│       │   ├── 0.jpg
│       │   ├── 1.jpg
│       │   └── ...
│       └── 2/         # 第2章
│           └── ...
└── config.json        # 配置文件
```

### 数据库表结构

#### comics 表

存储已下载的漫画信息。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT | 漫画ID（主键）|
| title | TEXT | 标题 |
| author | TEXT | 作者 |
| description | TEXT | 描述 |
| cover | TEXT | 封面URL |
| tags | TEXT | 标签（JSON数组）|
| categories | TEXT | 分类（JSON数组）|
| eps_count | INTEGER | 章节数 |
| pages_count | INTEGER | 总页数 |
| type | TEXT | 漫画类型 |
| time | INTEGER | 下载时间（Unix时间戳）|
| size | INTEGER | 文件大小（字节）|
| directory | TEXT | 存储目录名 |
| eps | TEXT | 章节列表（JSON数组）|
| downloaded_eps | TEXT | 已下载章节（JSON数组）|

#### download_tasks 表

存储下载任务信息。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT | 任务ID（主键）|
| comic_id | TEXT | 漫画ID |
| title | TEXT | 标题 |
| type | TEXT | 类型 |
| cover | TEXT | 封面URL |
| total_pages | INTEGER | 总页数 |
| downloaded_pages | INTEGER | 已下载页数 |
| current_ep | INTEGER | 当前章节 |
| status | TEXT | 状态 |
| error | TEXT | 错误信息 |
| created_at | INTEGER | 创建时间 |
| updated_at | INTEGER | 更新时间 |

## 客户端集成

客户端可以通过 HTTP API 与服务器通信。示例：

### JavaScript/TypeScript

```javascript
// 获取漫画列表
const response = await fetch('http://server-ip:8080/api/comics');
const data = await response.json();

// 添加下载任务
await fetch('http://server-ip:8080/api/download', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    type: 'picacg',
    comic_id: 'comic-id',
    eps: [1, 2, 3]
  })
});

// 获取漫画图片
const imageUrl = 'http://server-ip:8080/api/comics/comic-id/1/0';
```

### Dart/Flutter

```dart
// 获取漫画列表
final response = await http.get(Uri.parse('http://server-ip:8080/api/comics'));
final data = jsonDecode(response.body);

// 添加下载任务
await http.post(
  Uri.parse('http://server-ip:8080/api/download'),
  headers: {'Content-Type': 'application/json'},
  body: jsonEncode({
    'type': 'picacg',
    'comic_id': 'comic-id',
    'eps': [1, 2, 3],
  }),
);
```

## 开发

### 项目结构

```
server/
├── main.go                    # 主程序入口
├── go.mod                     # Go 模块文件
├── api/                       # API 路由
│   ├── routes.go             # 路由注册
│   └── handlers/             # 请求处理器
│       ├── comic_handler.go
│       ├── download_handler.go
│       └── picacg_handler.go
├── models/                    # 数据模型
│   └── comic.go
├── services/                  # 业务逻辑
│   └── download_manager.go
├── picacg/                    # PicaComic 客户端
│   └── client.go
└── README.md
```

### 扩展其他漫画源

要添加新的漫画源（如 E-Hentai、禁漫等），需要：

1. 在 `models/comic.go` 中添加对应的请求/响应模型
2. 创建新的客户端包（如 `ehentai/client.go`）
3. 在 `services/download_manager.go` 的 `downloadTask` 方法中添加新类型的处理
4. 在 `api/handlers/` 中添加对应的 API 处理器

## 注意事项

1. 服务器默认监听所有网络接口（0.0.0.0），请确保网络安全
2. 下载的漫画文件较大，请确保有足够的磁盘空间
3. PicaComic API 需要登录才能使用，请妥善保管账号信息
4. 建议在生产环境中使用反向代理（如 Nginx）并配置 HTTPS

## 许可证

本项目仅供学习交流使用。

