// 默认服务器地址
const DEFAULT_SERVER_URL = 'http://localhost:8080';

// 页面加载时恢复配置
document.addEventListener('DOMContentLoaded', () => {
  chrome.storage.local.get(['serverUrl'], (result) => {
    if (result.serverUrl) {
      document.getElementById('serverUrl').value = result.serverUrl;
    }
  });
});

// 保存配置
document.getElementById('saveConfigBtn').addEventListener('click', () => {
  const serverUrl = document.getElementById('serverUrl').value.trim();

  if (!serverUrl) {
    showStatus('请输入服务器地址', 'error');
    return;
  }

  // 验证 URL 格式
  try {
    new URL(serverUrl);
  } catch (e) {
    showStatus('服务器地址格式不正确', 'error');
    return;
  }

  chrome.storage.local.set({ serverUrl }, () => {
    showStatus('配置已保存', 'success');
  });
});

// 提取链接按钮
document.getElementById('extractBtn').addEventListener('click', async () => {
  const statusDiv = document.getElementById('status');
  const resultDiv = document.getElementById('result');

  // 重置状态
  resultDiv.classList.remove('show');

  showStatus('正在提取链接...', 'info');

  try {
    // 获取服务器地址
    const serverUrl = await getServerUrl();

    // 获取当前活动标签页
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab) {
      showStatus('无法获取当前标签页', 'error');
      return;
    }

    // 向 content script 发送消息,提取链接
    const response = await chrome.tabs.sendMessage(tab.id, { action: 'extractLinks' });

    if (!response) {
      showStatus('无法从页面提取链接,请刷新页面后重试', 'error');
      return;
    }

    if (!response.links || response.links.length === 0) {
      showStatus('当前页面未找到有效链接', 'error');
      return;
    }

    showStatus(`找到 ${response.links.length} 个链接,正在上传到服务器...`, 'info');

    // 发送到服务器
    const serverResponse = await fetch(`${serverUrl}/api/parse-from-extension`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        title: response.title || '未命名页面',
        links: response.links,
        sourceUrl: tab.url
      })
    });

    if (!serverResponse.ok) {
      const errorText = await serverResponse.text();
      showStatus(`服务器错误: ${errorText}`, 'error');
      return;
    }

    const result = await serverResponse.json();

    if (result.viewURL) {
      showStatus('生成成功!', 'success');
      resultDiv.innerHTML = `
        <div style="margin-bottom: 10px;">
          <strong>卡片页面已生成:</strong>
        </div>
        <a href="${result.viewURL}" target="_blank">${result.viewURL}</a>
        <div class="result-info">
          标题: ${result.title || '无标题'}<br>
          链接数量: ${result.count} 个
        </div>
      `;
      resultDiv.classList.add('show');
    } else {
      showStatus('生成失败: ' + (result.error || '未知错误'), 'error');
    }

  } catch (error) {
    console.error('提取链接失败:', error);
    if (error.message.includes('Could not establish connection')) {
      showStatus('无法连接到页面,请刷新页面后重试', 'error');
    } else if (error.message.includes('fetch')) {
      showStatus('无法连接到服务器,请检查服务器是否运行', 'error');
    } else {
      showStatus('错误: ' + error.message, 'error');
    }
  }
});

// 显示状态消息
function showStatus(message, type) {
  const statusDiv = document.getElementById('status');
  statusDiv.textContent = message;
  statusDiv.className = type;
}

// 获取服务器地址
function getServerUrl() {
  return new Promise((resolve) => {
    chrome.storage.local.get(['serverUrl'], (result) => {
      resolve(result.serverUrl || DEFAULT_SERVER_URL);
    });
  });
}
