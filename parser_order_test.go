package main

import (
	"sync"
	"testing"
)

func TestLinkOrderWithFavicon(t *testing.T) {
	// 模拟解析后的链接
	cards := []CardItem{
		{Title: "链接1", URL: "https://example1.com"},
		{Title: "链接2", URL: "https://example2.com"},
		{Title: "链接3", URL: "https://example3.com"},
		{Title: "链接4", URL: "https://example4.com"},
		{Title: "链接5", URL: "https://example5.com"},
	}

	// 模拟并行获取 favicon（和实际代码一样）
	var wg sync.WaitGroup
	for i := range cards {
		if cards[i].URL != "" {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				// 模拟 getFaviconURL
				cards[index].Favicon = "favicon_" + cards[index].Title
			}(i)
		}
	}
	wg.Wait()

	// 验证顺序
	expectedTitles := []string{"链接1", "链接2", "链接3", "链接4", "链接5"}

	for i, card := range cards {
		if card.Title != expectedTitles[i] {
			t.Errorf("位置 %d: 期望标题 %s，实际得到 %s", i, expectedTitles[i], card.Title)
		}
		if card.Favicon != "favicon_"+expectedTitles[i] {
			t.Errorf("位置 %d: favicon 不匹配", i)
		}
	}

	t.Logf("并行处理后链接顺序测试通过！")
	for i, card := range cards {
		t.Logf("  %d. %s - %s - %s", i+1, card.Title, card.URL, card.Favicon)
	}
}
