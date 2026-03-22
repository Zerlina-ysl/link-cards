package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ParsePage 解析页面内容，提取链接
func ParsePage(pageURL string, cookie string) ([]CardItem, string, error) {
	// 创建 HTTP 客户端，支持 Cookie
	client := &http.Client{}
	if cookie != "" {
		client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // 不自动重定向，保留 Cookie
			},
		}
	}

	// 创建请求
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, "", err
	}

	// 设置 User-Agent 模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 设置 Cookie
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	content := string(body)

	// 详细调试信息
	Log.Debugf("=== 响应信息 ===")
	Log.Debugf("状态码: %d", resp.StatusCode)
	Log.Debugf("parse url content: %v", content)
	

	// 检查 HTTP 状态码（非 200 且不是重定向）
	if resp.StatusCode >= 400 {
		Log.Warnf("可能需要认证: 状态码=%d", resp.StatusCode)
		return nil, "内容解析失败", fmt.Errorf("内容解析失败")	
	}

	// 提取标题
	title := extractTitle(content)

	// 尝试提取正文区域
	mainContent := extractMainContent(content)

	// 统一使用 Markdown 解析（适用于飞书、语雀、学城等平台）
	cards := parseMarkdownLinks(mainContent)

	// 并行获取每个链接的 favicon
	var wg sync.WaitGroup
	for i := range cards {
		if cards[i].URL != "" {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				cards[index].Favicon = getFaviconURL(cards[index].URL)
			}(i)
		}
	}
	wg.Wait()

	return cards, title, nil
}

// extractTitle 从内容中提取标题
func extractTitle(content string) string {
	// 尝试从 <title> 标签提取
	titleRegex := regexp.MustCompile(`<title>([^<]+)</title>`)
	if match := titleRegex.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	// 尝试从 Markdown 标题提取
	mdTitleRegex := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	if match := mdTitleRegex.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	return ""
}

// extractMainContent 提取页面正文区域
func extractMainContent(content string) string {
	// 尝试多种常见的正文容器标签
	patterns := []struct {
		name  string
		regex *regexp.Regexp
	}{
		{"<article>", regexp.MustCompile(`(?s)<article[^>]*>(.*?)</article>`)},
		{"<main>", regexp.MustCompile(`(?s)<main[^>]*>(.*?)</main>`)},
		{"role=main", regexp.MustCompile(`(?s)<[^>]*role="main"[^>]*>(.*?)</[^>]+>`)},
		{".doc-content", regexp.MustCompile(`(?s)<[^>]*class="[^"]*doc-content[^"]*"[^>]*>(.*?)</[^>]+>`)},
		{".content", regexp.MustCompile(`(?s)<[^>]*class="[^"]*\bcontent\b[^"]*"[^>]*>(.*?)</[^>]+>`)},
		{"#content", regexp.MustCompile(`(?s)<[^>]*id="content"[^>]*>(.*?)</[^>]+>`)},
	}

	for _, p := range patterns {
		if match := p.regex.FindStringSubmatch(content); len(match) > 1 {
			Log.Debugf("找到正文区域: %s", p.name)
			return match[1]
		}
	}

	Log.Debugf("未找到正文区域，使用完整内容")
	return content
}

// extractDomain 从 URL 提取域名
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}

// getFaviconURL 获取网站图标 URL，主动检测可用性（带缓存）
func getFaviconURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		Log.Debugf("解析 URL 失败: %s, error: %v", rawURL, err)
		return ""
	}

	domain := u.Host
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}

	// 检查缓存
	faviconCacheMu.RLock()
	if cached, ok := faviconCache[domain]; ok {
		faviconCacheMu.RUnlock()
		Log.Debugf("使用缓存图标: %s -> %s", domain, cached)
		return cached
	}
	faviconCacheMu.RUnlock()

	// 准备多个可能的图标源
	sources := []string{
		fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=256", domain),
		fmt.Sprintf("%s://%s/favicon.ico", scheme, domain),
	}

	Log.Debugf("开始检测图标可用性: %s", rawURL)

	// 并行检测所有源，返回第一个可用的
	resultChan := make(chan string, len(sources))

	for _, src := range sources {
		go func(iconURL string) {
			if checkFaviconAvailable(iconURL) {
				Log.Debugf("图标可用: %s", iconURL)
				resultChan <- iconURL
			} else {
				Log.Debugf("图标不可用: %s", iconURL)
				resultChan <- ""
			}
		}(src)
	}

	// 等待所有检测完成，返回第一个可用的
	var result string
	for i := 0; i < len(sources); i++ {
		if r := <-resultChan; r != "" && result == "" {
			result = r
		}
	}

	// 缓存结果（无论成功还是失败）
	faviconCacheMu.Lock()
	faviconCache[domain] = result
	faviconCacheMu.Unlock()

	if result != "" {
		Log.Debugf("使用图标: %s", result)
	} else {
		Log.Debugf("所有图标源都不可用，将使用默认图标: %s", rawURL)
	}

	return result
}

// 图标检测缓存（域名 -> 图标 URL）
var faviconCache = make(map[string]string)
var faviconCacheMu sync.RWMutex

// checkFaviconAvailable 检测图标 URL 是否可访问
func checkFaviconAvailable(iconURL string) bool {
	// 创建带超时的 HTTP 客户端（300ms 超时）
	client := &http.Client{
		Timeout: 300 * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 允许重定向
			return nil
		},
	}

	// 使用 HEAD 请求，只检查可用性，不下载内容
	req, err := http.NewRequest("HEAD", iconURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 检查状态码是否为 2xx
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// 额外检查 Content-Type 是否为图片
		contentType := resp.Header.Get("Content-Type")
		if contentType != "" && (
			strings.HasPrefix(contentType, "image/") ||
			contentType == "application/octet-stream") {
			return true
		}
	}

	return false
}

// parseMarkdownLinks 解析 Markdown 格式的链接
func parseMarkdownLinks(content string) []CardItem {
	var cards []CardItem
	seen := make(map[string]bool)

	// 匹配 Markdown 链接格式: [文本](URL)
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := linkRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		linkText := strings.TrimSpace(match[1])
		linkURL := strings.TrimSpace(match[2])

		// 跳过图片和无效链接
		if strings.HasPrefix(linkURL, "!") || !strings.HasPrefix(linkURL, "http") {
			continue
		}

		// 去重
		if seen[linkURL] {
			continue
		}
		seen[linkURL] = true

		cards = append(cards, CardItem{
			Title: linkText,
			URL:   linkURL,
		})
	}

	// 如果没有 Markdown 链接，尝试解析 HTML 链接
	if len(cards) == 0 {
		cards = parseHTMLLinks(content)
	}

	return cards
}

// parseHTMLLinks 解析 HTML 格式的链接
func parseHTMLLinks(content string) []CardItem {
	var cards []CardItem
	seen := make(map[string]bool)

	// 匹配 HTML 链接格式: <a href="URL">文本</a>
	linkRegex := regexp.MustCompile(`href="([^"]+)"[^>]*>([^<]*)</a>`)
	matches := linkRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		linkURL := strings.TrimSpace(match[1])
		linkText := strings.TrimSpace(match[2])

		if !strings.HasPrefix(linkURL, "http") {
			continue
		}

		if seen[linkURL] {
			continue
		}
		seen[linkURL] = true

		// 清理 HTML 实体
		linkText = strings.ReplaceAll(linkText, "&amp;", "&")
		linkText = strings.ReplaceAll(linkText, "&lt;", "<")
		linkText = strings.ReplaceAll(linkText, "&gt;", ">")
		linkText = strings.ReplaceAll(linkText, "&quot;", "\"")
		linkText = strings.ReplaceAll(linkText, "&#39;", "'")

		cards = append(cards, CardItem{
			Title: linkText,
			URL:   linkURL,
		})
	}

	return cards
}
