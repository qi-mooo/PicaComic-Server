package picacg

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	apiURL     = "https://picaapi.picacomic.com"
	apiKey     = "C69BAF41DA5ABD1FFEDC6D2FEA56B"
	secretKey  = "~d}$Q7$eIni=V)9\\RK/P.RM4;9[7|@/CA}b~OW!3?EV`:<>M7pddUBL5n|0/*Cn"
	apiVersion = "2.2.1.3.3.4"
	buildVer   = "45"
)

// Client PicaComic API 客户端
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient 创建新的客户端
func NewClient() *Client {
	// 禁用 HTTP/2，只使用 HTTP/1.1
	transport := &http.Transport{
		ForceAttemptHTTP2: false,
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// SetToken 设置认证 token
func (c *Client) SetToken(token string) {
	c.token = token
}

// Login 登录
func (c *Client) Login(email, password string) (string, error) {
	data := map[string]string{
		"email":    email,
		"password": password,
	}

	resp, err := c.post("/auth/sign-in", data)
	if err != nil {
		return "", err
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	if result.Message != "success" {
		return "", fmt.Errorf("登录失败: %s", result.Message)
	}

	c.token = result.Data.Token
	return c.token, nil
}

// GetCategories 获取分类
func (c *Client) GetCategories() (interface{}, error) {
	resp, err := c.get("/categories")
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result["data"], nil
}

// Search 搜索漫画
func (c *Client) Search(keyword, sort string, page int) (interface{}, error) {
	data := map[string]string{
		"keyword": keyword,
		"sort":    sort,
	}

	url := fmt.Sprintf("/comics/advanced-search?page=%d", page)
	resp, err := c.post(url, data)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result["data"], nil
}

// GetComicInfo 获取漫画信息
func (c *Client) GetComicInfo(id string) (interface{}, error) {
	url := fmt.Sprintf("/comics/%s", id)
	resp, err := c.get(url)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result["data"], nil
}

// GetEps 获取章节
func (c *Client) GetEps(id string) ([]string, error) {
	var eps []string
	page := 1

	for {
		url := fmt.Sprintf("/comics/%s/eps?page=%d", id, page)
		resp, err := c.get(url)
		if err != nil {
			return nil, err
		}

		var result struct {
			Data struct {
				Eps struct {
					Docs []struct {
						Title string `json:"title"`
					} `json:"docs"`
					Pages int `json:"pages"`
				} `json:"eps"`
			} `json:"data"`
		}

		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, err
		}

		for _, doc := range result.Data.Eps.Docs {
			eps = append(eps, doc.Title)
		}

		if page >= result.Data.Eps.Pages {
			break
		}
		page++
	}

	// 反转数组（PicaComic 返回的是倒序）
	for i, j := 0, len(eps)-1; i < j; i, j = i+1, j-1 {
		eps[i], eps[j] = eps[j], eps[i]
	}

	return eps, nil
}

// GetComicPages 获取漫画图片链接
func (c *Client) GetComicPages(id string, order int) ([]string, error) {
	var images []string
	page := 1

	for {
		url := fmt.Sprintf("/comics/%s/order/%d/pages?page=%d", id, order, page)
		fmt.Printf("[API] 请求图片列表: %s\n", url)
		resp, err := c.get(url)
		if err != nil {
			fmt.Printf("[API错误] 请求失败: %v\n", err)
			return nil, err
		}

		fmt.Printf("[API] 响应长度: %d 字节\n", len(resp))
		fmt.Printf("[API] 响应内容: %s\n", string(resp)[:min(500, len(resp))])

		var result struct {
			Data struct {
				Pages struct {
					Docs []struct {
						Media struct {
							FileServer string `json:"fileServer"`
							Path       string `json:"path"`
						} `json:"media"`
					} `json:"docs"`
					Pages int `json:"pages"`
				} `json:"pages"`
			} `json:"data"`
		}

		if err := json.Unmarshal(resp, &result); err != nil {
			fmt.Printf("[API错误] JSON 解析失败: %v\n", err)
			return nil, err
		}

		fmt.Printf("[API] 本页图片数: %d, 总页数: %d\n", len(result.Data.Pages.Docs), result.Data.Pages.Pages)

		for _, doc := range result.Data.Pages.Docs {
			imageURL := doc.Media.FileServer + "/static/" + doc.Media.Path
			images = append(images, imageURL)
		}

		if page >= result.Data.Pages.Pages {
			break
		}
		page++
	}

	fmt.Printf("[API] 共获取 %d 张图片\n", len(images))
	return images, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *Client) get(path string) ([]byte, error) {
	return c.request("GET", path, nil)
}

func (c *Client) post(path string, data interface{}) ([]byte, error) {
	return c.request("POST", path, data)
}

func (c *Client) request(method, path string, data interface{}) ([]byte, error) {
	url := apiURL + path

	var body []byte
	var err error
	if data != nil {
		body, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// 设置请求头
	c.setHeaders(req, method, path)

	// 打印完整请求头
	fmt.Printf("[请求] %s %s\n", method, url)
	fmt.Printf("[请求头]:\n")
	for k, v := range req.Header {
		if k == "authorization" {
			fmt.Printf("  %s: %s...\n", k, v[0][:min(50, len(v[0]))])
		} else {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("[请求错误]: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Printf("[响应] HTTP %d, 内容长度: %d\n", resp.StatusCode, len(respBody))

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) setHeaders(req *http.Request, method, path string) {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := strings.Replace(uuid.New().String(), "-", "", -1)

	raw := path + now + nonce + method + apiKey
	raw = strings.ToLower(raw)

	fmt.Printf("[签名] 原始字符串: %s\n", raw[:min(100, len(raw))])

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(raw))
	signature := hex.EncodeToString(mac.Sum(nil))

	fmt.Printf("[签名] 生成的签名: %s\n", signature)

	// 注意：必须使用小写，Picacg API 对大小写敏感
	req.Header["api-key"] = []string{apiKey}
	req.Header["accept"] = []string{"application/vnd.picacomic.com.v1+json"}
	req.Header["app-channel"] = []string{"3"}
	req.Header["time"] = []string{now}
	req.Header["nonce"] = []string{nonce}
	req.Header["signature"] = []string{signature}
	req.Header["app-version"] = []string{apiVersion}
	req.Header["app-uuid"] = []string{"defaultUuid"}
	req.Header["image-quality"] = []string{"original"}
	req.Header["app-platform"] = []string{"android"}
	req.Header["app-build-version"] = []string{buildVer}
	req.Header["Content-Type"] = []string{"application/json; charset=UTF-8"}
	req.Header["user-agent"] = []string{"okhttp/3.8.1"}
	req.Header["version"] = []string{"v1.4.1"}
	req.Header["Host"] = []string{"picaapi.picacomic.com"}

	if c.token != "" {
		req.Header["authorization"] = []string{c.token}
		fmt.Printf("[认证] Token 已设置: %s...\n", c.token[:min(50, len(c.token))])
	} else {
		fmt.Printf("[警告] Token 未设置!\n")
	}
}
