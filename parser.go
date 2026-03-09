package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
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

	// 统一使用 Markdown 解析（适用于飞书、语雀、学城等平台）
	cards := parseMarkdownLinks(content)

	// 获取每个链接的 favicon (使用多个备用源)
	for i := range cards {
		if cards[i].URL != "" {
			cards[i].Favicon = getFaviconURL(cards[i].URL)
		}
	}

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

// extractDomain 从 URL 提取域名
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}

// getFaviconURL 获取网站图标 URL (使用多个备用源)
func getFaviconURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	domain := u.Host
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}

	// 使用 Google favicon 服务，请求更大尺寸以保证清晰度
	// sz=256 可以获取更高清的图标
	return fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=256", domain)
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

// 保留旧接口以兼容
func parseKMContent(content string) ([]CardItem, string) {
	title := extractTitle(content)
	cards := parseMarkdownLinks(content)
	return cards, title
}

func parseFeishuContent(content string) ([]CardItem, string) {
	return parseKMContent(content)
}

func parseYuqueContent(content string) ([]CardItem, string) {
	return parseKMContent(content)
}

func parseGenericContent(content string) ([]CardItem, string) {
	return parseKMContent(content)
}
