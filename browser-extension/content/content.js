// 监听来自 popup 的消息
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'extractLinks') {
    try {
      const links = extractLinksFromPage();
      const title = extractPageTitle();

      console.log('[Link Cards] 提取到链接数量:', links.length);
      console.log('[Link Cards] 页面标题:', title);

      sendResponse({ links, title });
    } catch (error) {
      console.error('[Link Cards] 提取链接失败:', error);
      sendResponse({ links: [], title: '', error: error.message });
    }
  }
  return true; // 保持消息通道开启
});

// 提取页面标题
function extractPageTitle() {
  // 优先使用 document.title
  if (document.title) {
    return document.title;
  }

  // 尝试从 h1 标签提取
  const h1 = document.querySelector('h1');
  if (h1 && h1.textContent.trim()) {
    return h1.textContent.trim();
  }

  // 尝试从 meta 标签提取
  const metaTitle = document.querySelector('meta[property="og:title"]');
  if (metaTitle && metaTitle.content) {
    return metaTitle.content;
  }

  return '未命名页面';
}

// 提取页面中的链接
function extractLinksFromPage() {
  const links = [];
  const seen = new Set();

  // 方法1: 提取所有 <a> 标签
  const anchorLinks = extractAnchorLinks(seen);
  links.push(...anchorLinks);

  // 方法2: 提取 Markdown 格式的链接 (针对飞书、语雀等富文本编辑器)
  const markdownLinks = extractMarkdownLinks(seen);
  links.push(...markdownLinks);

  // 方法3: 针对特定平台的特殊处理
  const platformLinks = extractPlatformSpecificLinks(seen);
  links.push(...platformLinks);

  console.log('[Link Cards] 提取统计 - 锚点链接:', anchorLinks.length,
              'Markdown链接:', markdownLinks.length,
              '平台特殊链接:', platformLinks.length);

  return links;
}

// 智能提取链接文本（处理表格、列表等场景）
function extractLinkText(linkElement) {
  // 检查链接是否在表格单元格中
  const tdParent = linkElement.closest('td');
  if (tdParent) {
    // 如果在表格中，获取所在行
    const trParent = tdParent.closest('tr');
    if (trParent) {
      // 获取该行的所有单元格
      const cells = trParent.querySelectorAll('td, th');

      // 如果有多列，取第一列的文本作为标题
      if (cells.length > 1) {
        const firstCell = cells[0];
        const firstCellText = firstCell.textContent.trim();

        // 如果第一列有内容且不是链接本身，使用第一列
        if (firstCellText && firstCellText !== linkElement.textContent.trim()) {
          return firstCellText;
        }
      }
    }

    // 如果没有多列或第一列是链接本身，返回链接文本
    return linkElement.textContent.trim();
  }

  // 检查链接是否在列表项中
  const liParent = linkElement.closest('li');
  if (liParent) {
    // 如果在列表中，尝试提取列表项的第一个文本节点或第一个子元素
    const firstTextNode = Array.from(liParent.childNodes).find(
      node => node.nodeType === Node.TEXT_NODE && node.textContent.trim()
    );
    if (firstTextNode) {
      return firstTextNode.textContent.trim();
    }

    // 如果列表项包含多个链接，只取当前链接的文本
    const links = liParent.querySelectorAll('a');
    if (links.length > 1) {
      return linkElement.textContent.trim();
    }
  }

  // 检查链接是否在 div 或 span 容器中，且容器有多个子元素（可能是多列布局）
  const parent = linkElement.parentElement;
  if (parent && (parent.tagName === 'DIV' || parent.tagName === 'SPAN')) {
    const siblings = Array.from(parent.children);
    if (siblings.length > 1) {
      // 如果有多个子元素，只取链接自身的文本
      return linkElement.textContent.trim();
    }
  }

  // 默认返回链接的文本内容
  return linkElement.textContent.trim();
}

// 提取 <a> 标签链接
function extractAnchorLinks(seen) {
  const links = [];

  // 尝试找到正文区域
  const mainContent = findMainContent();

  // 优先从正文区域提取链接，如果找不到正文则从整个页面提取
  const anchors = mainContent ?
    mainContent.querySelectorAll('a[href]') :
    document.querySelectorAll('a[href]');

  anchors.forEach(link => {
    const url = link.href;
    let text = '';

    // 过滤无效链接
    if (!isValidUrl(url) || seen.has(url)) {
      return;
    }

    // 过滤导航栏、侧边栏、页脚等区域的链接
    if (shouldSkipLink(link)) {
      return;
    }

    // 智能提取链接文本
    text = extractLinkText(link);

    // 如果文本为空,尝试从 title 或 aria-label 获取
    if (!text) {
      text = link.getAttribute('title') || link.getAttribute('aria-label') || '';
    }

    // 清理文本(移除多余空白)
    text = text.replace(/\s+/g, ' ').trim();

    // 如果文本是 URL 或者太长,尝试提取更友好的名称
    if (!text || text === url || text.startsWith('http://') || text.startsWith('https://')) {
      text = extractFriendlyName(url);
    }

    // 限制文本长度
    if (text.length > 100) {
      text = text.substring(0, 100) + '...';
    }

    seen.add(url);
    links.push({
      title: text,
      url: url,
      remark: ''
    });
  });

  return links;
}

// 查找页面正文区域
function findMainContent() {
  // 尝试多种常见的正文容器选择器
  const selectors = [
    // 标准 HTML5 语义标签
    'article',
    'main',
    '[role="main"]',

    // 学城（美团内部）
    '.doc-content',
    '.km-doc-content',
    '#doc-content',

    // 飞书
    '.doc-content',
    '.lark-doc-content',

    // 语雀
    '.ne-viewer-body',
    '.lake-content',

    // Notion
    '.notion-page-content',

    // 通用
    '.content',
    '.main-content',
    '.article-content',
    '#content',
    '#main-content'
  ];

  for (const selector of selectors) {
    const element = document.querySelector(selector);
    if (element) {
      console.log('[Link Cards] 找到正文区域:', selector);
      return element;
    }
  }

  console.log('[Link Cards] 未找到正文区域，使用整个页面');
  return null;
}

// 判断是否应该跳过该链接
function shouldSkipLink(element) {
  // 检查元素本身和父元素的属性
  let current = element;
  let depth = 0;
  const maxDepth = 10; // 最多向上检查 10 层

  while (current && depth < maxDepth) {
    // 检查元素的 class、id、role 等属性
    const className = current.className ? current.className.toLowerCase() : '';
    const id = current.id ? current.id.toLowerCase() : '';
    const role = current.getAttribute('role') || '';
    const tagName = current.tagName.toLowerCase();

    // 需要跳过的区域关键词
    const skipPatterns = [
      // 导航相关
      'nav', 'navbar', 'navigation', 'menu', 'sidebar', 'side-bar',
      // 页眉页脚
      'header', 'footer', 'topbar', 'top-bar', 'bottom','scrollbar',
      // 面包屑和目录
      'breadcrumb', 'toc', 'table-of-contents', 'catalog',
      // 工具栏和按钮区
      'toolbar', 'tool-bar', 'action', 'button-group',
      // 其他
      'advertisement', 'ad-', 'banner', 'social', 'share'
    ];

    // 检查是否匹配跳过模式
    const textToCheck = `${className} ${id} ${role} ${tagName}`;
    for (const pattern of skipPatterns) {
      if (textToCheck.includes(pattern)) {
        return true;
      }
    }

    // 检查 role 属性
    if (role === 'navigation' || role === 'banner' || role === 'complementary') {
      return true;
    }

    // 检查特定标签
    if (tagName === 'nav' || tagName === 'header' || tagName === 'footer') {
      return true;
    }

    current = current.parentElement;
    depth++;
  }

  return false;
}

// 提取 Markdown 格式的链接
function extractMarkdownLinks(seen) {
  const links = [];
  const bodyText = document.body.innerText;

  // 匹配 [文本](URL) 格式
  const markdownLinkRegex = /\[([^\]]+)\]\(([^)]+)\)/g;
  let match;

  while ((match = markdownLinkRegex.exec(bodyText)) !== null) {
    const text = match[1].trim();
    const url = match[2].trim();

    if (isValidUrl(url) && !seen.has(url)) {
      seen.add(url);
      links.push({
        title: text || url,
        url: url,
        remark: ''
      });
    }
  }

  return links;
}

// 针对特定平台的链接提取
function extractPlatformSpecificLinks(seen) {
  const links = [];
  const hostname = window.location.hostname;

  // 飞书文档
  if (hostname.includes('feishu.cn') || hostname.includes('larksuite.com')) {
    const feishuLinks = extractFeishuLinks(seen);
    links.push(...feishuLinks);
  }

  // 语雀文档
  if (hostname.includes('yuque.com')) {
    const yuqueLinks = extractYuqueLinks(seen);
    links.push(...yuqueLinks);
  }

  // Notion
  if (hostname.includes('notion.so') || hostname.includes('notion.site')) {
    const notionLinks = extractNotionLinks(seen);
    links.push(...notionLinks);
  }

  return links;
}

// 飞书文档链接提取
function extractFeishuLinks(seen) {
  const links = [];

  // 飞书的链接通常在特定的 DOM 结构中
  const feishuLinkSelectors = [
    'a.link-card',
    'a[data-type="link"]',
    '.doc-link a'
  ];

  feishuLinkSelectors.forEach(selector => {
    const elements = document.querySelectorAll(selector);
    elements.forEach(el => {
      const url = el.href;
      const text = el.textContent.trim();

      if (isValidUrl(url) && !seen.has(url) && text) {
        seen.add(url);
        links.push({
          title: text,
          url: url,
          remark: ''
        });
      }
    });
  });

  return links;
}

// 语雀文档链接提取
function extractYuqueLinks(seen) {
  const links = [];

  // 语雀的链接选择器
  const yuqueLinkSelectors = [
    '.ne-link',
    'a.lake-link'
  ];

  yuqueLinkSelectors.forEach(selector => {
    const elements = document.querySelectorAll(selector);
    elements.forEach(el => {
      const url = el.href;
      const text = el.textContent.trim();

      if (isValidUrl(url) && !seen.has(url) && text) {
        seen.add(url);
        links.push({
          title: text,
          url: url,
          remark: ''
        });
      }
    });
  });

  return links;
}

// Notion 文档链接提取
function extractNotionLinks(seen) {
  const links = [];

  // Notion 的链接选择器
  const notionLinkSelectors = [
    'a.notion-link',
    'a[rel="noopener noreferrer"]'
  ];

  notionLinkSelectors.forEach(selector => {
    const elements = document.querySelectorAll(selector);
    elements.forEach(el => {
      const url = el.href;
      const text = el.textContent.trim();

      if (isValidUrl(url) && !seen.has(url) && text) {
        seen.add(url);
        links.push({
          title: text,
          url: url,
          remark: ''
        });
      }
    });
  });

  return links;
}

// 从 URL 提取友好的名称
function extractFriendlyName(url) {
  try {
    const urlObj = new URL(url);
    const hostname = urlObj.hostname;
    const pathname = urlObj.pathname;

    // 移除 www. 前缀
    const domain = hostname.replace(/^www\./, '');

    // 如果有路径,尝试从路径提取有意义的部分
    if (pathname && pathname !== '/' && pathname !== '') {
      const pathParts = pathname.split('/').filter(p => p && p.length > 0);

      // 如果路径的最后一部分看起来像有意义的名称
      if (pathParts.length > 0) {
        const lastPart = pathParts[pathParts.length - 1];

        // 移除文件扩展名
        const nameWithoutExt = lastPart.replace(/\.(html|htm|php|asp|aspx|jsp)$/i, '');

        // 如果是有意义的名称(不是纯数字或ID)
        if (nameWithoutExt.length > 2 && !/^\d+$/.test(nameWithoutExt)) {
          // 将连字符和下划线替换为空格,首字母大写
          const friendlyName = nameWithoutExt
            .replace(/[-_]/g, ' ')
            .split(' ')
            .map(word => word.charAt(0).toUpperCase() + word.slice(1))
            .join(' ');

          return friendlyName.length < 50 ? friendlyName : domain;
        }
      }
    }

    // 默认返回域名
    return domain;
  } catch (e) {
    return url;
  }
}

// 验证 URL 是否有效
function isValidUrl(url) {
  if (!url) return false;

  // 必须是 http 或 https 开头
  if (!url.startsWith('http://') && !url.startsWith('https://')) {
    return false;
  }

  // 排除一些常见的无效链接
  const invalidPatterns = [
    /javascript:/i,
    /^\s*$/,
    /#$/,  // 只有锚点的链接
    /^about:/i,
    /^chrome:/i,
    /^chrome-extension:/i
  ];

  for (const pattern of invalidPatterns) {
    if (pattern.test(url)) {
      return false;
    }
  }

  return true;
}
