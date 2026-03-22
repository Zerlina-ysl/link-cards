package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// CardItem 链接卡片
type CardItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Remark  string `json:"remark"`
	Favicon string `json:"favicon"`
}

// CardData 卡片页面数据
type CardData struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Cards   []CardItem `json:"cards"`
}

var (
	cardStore = make(map[string]*CardData)
	storeMu   sync.RWMutex
)

// 生成随机 ID
func generateID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// HandleParse 处理解析请求
func HandleParse(c *gin.Context) {
	var req struct {
		URL    string `json:"url" binding:"required"`
		Title  string `json:"title"`
		Cookie string `json:"cookie"` // 可选，用于需要认证的链接
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 URL"})
		return
	}

	// 解析页面内容
	cards, title, err := ParsePage(req.URL, req.Cookie)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析失败: " + err.Error()})
		return
	}
	Log.Debugf("URL parse succ, cards:%v, title:%s", cards, title)

	// 使用用户提供的标题或从页面提取的标题
	pageTitle := req.Title
	if pageTitle == "" && title != "" {
		pageTitle = title
	}

	// 生成唯一 ID
	id := generateID()

	// 存储数据
	storeMu.Lock()
	cardStore[id] = &CardData{
		ID:      id,
		Title:   pageTitle,
		Cards:   cards,
	}
	storeMu.Unlock()

	// 返回访问链接
	viewURL := c.Request.Host + "/view/" + id
	protocol := "http"
	if c.Request.TLS != nil {
		protocol = "https"
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"viewURL": protocol + "://" + viewURL,
		"title":   pageTitle,
		"count":   len(cards),
	})
}

// HandleList 处理列表请求（用于管理）
func HandleList(c *gin.Context) {
	storeMu.RLock()
	defer storeMu.RUnlock()

	list := make([]*CardData, 0, len(cardStore))
	for _, v := range cardStore {
		list = append(list, v)
	}

	c.JSON(http.StatusOK, gin.H{"data": list})
}

// GetCardData 获取卡片数据
func GetCardData(id string) *CardData {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return cardStore[id]
}

// HandleParseFromExtension 处理来自浏览器扩展的解析请求
func HandleParseFromExtension(c *gin.Context) {
	var req struct {
		Title     string     `json:"title"`
		Links     []CardItem `json:"links"`
		SourceURL string     `json:"sourceUrl"` // 可选，记录来源页面
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "数据格式错误"})
		return
	}

	// 验证链接数据
	if len(req.Links) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未提供任何链接"})
		return
	}

	Log.Debugf("收到扩展数据: title=%s, links=%d, source=%s", req.Title, len(req.Links), req.SourceURL)

	// 并行为每个链接添加 favicon
	var wg sync.WaitGroup
	for i := range req.Links {
		if req.Links[i].URL != "" {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				req.Links[index].Favicon = getFaviconURL(req.Links[index].URL)
			}(i)
		}
	}
	wg.Wait()

	// 生成唯一 ID
	id := generateID()

	// 存储数据
	storeMu.Lock()
	cardStore[id] = &CardData{
		ID:    id,
		Title: req.Title,
		Cards: req.Links,
	}
	storeMu.Unlock()

	Log.Debugf("扩展数据存储成功: id=%s", id)

	// 返回访问链接
	viewURL := c.Request.Host + "/view/" + id
	protocol := "http"
	if c.Request.TLS != nil {
		protocol = "https"
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"viewURL": protocol + "://" + viewURL,
		"title":   req.Title,
		"count":   len(req.Links),
	})
}
