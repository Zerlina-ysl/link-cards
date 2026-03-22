package main

import (
	"testing"
)

func TestLinkOrder(t *testing.T) {
	content := `
# 测试链接顺序

[链接1](https://example1.com)
[链接2](https://example2.com)
[链接3](https://example3.com)
[链接4](https://example4.com)
[链接5](https://example5.com)
`

	cards := parseMarkdownLinks(content)

	expectedTitles := []string{"链接1", "链接2", "链接3", "链接4", "链接5"}
	expectedURLs := []string{
		"https://example1.com",
		"https://example2.com",
		"https://example3.com",
		"https://example4.com",
		"https://example5.com",
	}

	if len(cards) != len(expectedTitles) {
		t.Fatalf("期望 %d 个链接，实际得到 %d 个", len(expectedTitles), len(cards))
	}

	for i, card := range cards {
		if card.Title != expectedTitles[i] {
			t.Errorf("位置 %d: 期望标题 %s，实际得到 %s", i, expectedTitles[i], card.Title)
		}
		if card.URL != expectedURLs[i] {
			t.Errorf("位置 %d: 期望 URL %s，实际得到 %s", i, expectedURLs[i], card.URL)
		}
	}

	t.Logf("链接顺序测试通过！")
	for i, card := range cards {
		t.Logf("  %d. %s - %s", i+1, card.Title, card.URL)
	}
}
