package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AccountCredentials 漫画源账号凭据
type AccountCredentials struct {
	Source      string                 `json:"source"`      // picacg, ehentai, jm, etc
	Credentials map[string]interface{} `json:"credentials"` // 源特定的凭据信息
}

// SyncAccountsRequest 同步账号请求
type SyncAccountsRequest struct {
	Accounts []AccountCredentials `json:"accounts"`
}

// 全局账号存储（实际应该存储到数据库）
var accountStore = make(map[string]map[string]interface{})

// SyncAccounts 同步客户端的账号信息到服务器
func SyncAccounts(c *gin.Context) {
	var req SyncAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新账号信息
	for _, account := range req.Accounts {
		accountStore[account.Source] = account.Credentials
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "账号同步成功",
		"synced":  len(req.Accounts),
	})
}

// GetAccounts 获取已同步的账号列表
func GetAccounts(c *gin.Context) {
	accounts := make([]AccountCredentials, 0)
	for source, creds := range accountStore {
		accounts = append(accounts, AccountCredentials{
			Source:      source,
			Credentials: creds,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts": accounts,
	})
}

// GetAccountBySource 获取特定漫画源的账号信息
func GetAccountBySource(c *gin.Context) {
	source := c.Param("source")

	if creds, exists := accountStore[source]; exists {
		c.JSON(http.StatusOK, gin.H{
			"source":      source,
			"credentials": creds,
		})
	} else {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "未找到该漫画源的账号信息",
		})
	}
}

// GetPicacgAccount 获取 PicaComic 账号凭据（内部使用）
func GetPicacgAccount() (map[string]interface{}, error) {
	if creds, exists := accountStore["picacg"]; exists {
		return creds, nil
	}
	return nil, fmt.Errorf("未找到 PicaComic 账号")
}

// GetJMAccount 获取 JM 账号凭据（内部使用）
func GetJMAccount() (map[string]interface{}, error) {
	if creds, exists := accountStore["jm"]; exists {
		return creds, nil
	}
	return nil, fmt.Errorf("未找到 JM 账号")
}

// GetEHAccount 获取 EHentai 账号凭据（内部使用）
func GetEHAccount() (map[string]interface{}, error) {
	if creds, exists := accountStore["ehentai"]; exists {
		return creds, nil
	}
	return nil, fmt.Errorf("未找到 EHentai 账号")
}
