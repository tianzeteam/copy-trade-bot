# Copy Trade Bot 国际化实现方案

## 概述
为 Copy Trade Bot Web 管理界面添加中英文双语支持。

## 已完成
1. ✅ 创建 i18n.js 国际化框架
2. ✅ 定义中英文翻译配置

## 实现步骤

### 1. 更新 HTML 页面

需要更新以下页面：
- `login.html` - 登录页面
- `index.html` - 钱包管理页面  
- `dashboard.html` - 仪表板页面

#### 更新方法：
在需要翻译的元素上添加 `data-i18n` 属性：

```html
<!-- 原来 -->
<h1>Copy Trade Bot - 聪明钱包管理</h1>

<!-- 更新后 -->
<h1 data-i18n="app.title">Copy Trade Bot - 聪明钱包管理</h1>
```

#### 特殊属性：
- `data-i18n` - 更新元素文本内容
- `data-i18n-placeholder` - 更新 placeholder 属性
- `data-i18n-title` - 更新 title 属性

### 2. 添加语言切换按钮

在页面顶部添加语言切换组件：

```html
<div class="language-switcher">
    <button class="btn btn-sm btn-outline-secondary" data-lang-switch="zh">
        中文
    </button>
    <button class="btn btn-sm btn-outline-secondary" data-lang-switch="en">
        English
    </button>
</div>
```

### 3. 引入 i18n.js

在所有 HTML 页面的 `<head>` 或 `<body>` 底部引入：

```html
<script src="/static/js/i18n.js"></script>
```

### 4. 更新 JavaScript 代码

在动态生成内容时使用 i18n：

```javascript
// 原来
alert('操作成功');

// 更新后
alert(i18n.t('msg.operation_success'));
```

### 5. CSS 样式

添加语言切换按钮样式：

```css
.language-switcher {
    position: fixed;
    top: 20px;
    right: 20px;
    z-index: 1000;
}

.language-switcher button {
    margin-left: 5px;
}

.language-switcher button.active {
    background-color: #0d6efd;
    color: white;
    border-color: #0d6efd;
}
```

## 翻译键值对照表

### 登录页面 (login.html)
| 元素 | 翻译键 | 中文 | 英文 |
|------|--------|------|------|
| 页面标题 | login.title | 登录 | Login |
| 用户名 | login.username | 用户名 | Username |
| 密码 | login.password | 密码 | Password |
| 记住我 | login.remember | 记住我 | Remember Me |
| 登录按钮 | login.submit | 登录 | Login |

### 钱包管理页面 (index.html)
| 元素 | 翻译键 | 中文 | 英文 |
|------|--------|------|------|
| 页面标题 | app.title | Copy Trade Bot - 聪明钱包管理 | Copy Trade Bot - Smart Wallet Management |
| 钱包列表 | wallet.title | 钱包列表 | Wallet List |
| 添加钱包 | wallet.add | 添加钱包 | Add Wallet |
| 选择钱包 | wallet.select | 选择监控钱包 | Select Monitoring Wallet |
| 钱包名称 | wallet.name | 钱包名称 | Wallet Name |
| 钱包地址 | wallet.address | 钱包地址 | Wallet Address |
| 止盈百分比 | trade.take_profit | 止盈百分比 | Take Profit % |
| 止损百分比 | trade.stop_loss | 止损百分比 | Stop Loss % |
| 持仓时间 | trade.holding_time | 持仓时间 | Holding Time |

### 仪表板页面 (dashboard.html)
| 元素 | 翻译键 | 中文 | 英文 |
|------|--------|------|------|
| 仪表板 | nav.dashboard | 仪表板 | Dashboard |
| 钱包管理 | nav.wallets | 钱包管理 | Wallets |
| 设置 | nav.settings | 设置 | Settings |
| 总交易数 | stats.total_trades | 总交易数 | Total Trades |
| 成功率 | stats.success_rate | 成功率 | Success Rate |
| 总收益 | stats.total_profit | 总收益 | Total Profit |

## 使用示例

### HTML 中使用
```html
<h1 data-i18n="app.title"></h1>
<button data-i18n="app.save"></button>
<input type="text" data-i18n-placeholder="wallet.address">
```

### JavaScript 中使用
```javascript
// 获取翻译文本
const title = i18n.t('app.title');

// 显示提示消息
alert(i18n.t('msg.operation_success'));

// 确认对话框
if (confirm(i18n.t('msg.confirm_delete'))) {
    // 删除操作
}

// 动态创建元素
const button = document.createElement('button');
button.textContent = i18n.t('app.save');
```

### 监听语言变更事件
```javascript
window.addEventListener('languageChanged', (event) => {
    console.log('Language changed to:', event.detail.lang);
    // 重新加载数据或更新界面
});
```

## 测试清单

- [ ] 登录页面中英文切换
- [ ] 钱包管理页面中英文切换
- [ ] 仪表板页面中英文切换
- [ ] 语言选择持久化（刷新后保持）
- [ ] 动态内容翻译
- [ ] 表单验证消息翻译
- [ ] Toast 通知消息翻译
- [ ] 确认对话框翻译

## 注意事项

1. **翻译键命名规范**：使用点号分隔的命名空间，如 `wallet.add`、`trade.title`
2. **默认语言**：默认使用中文，首次访问时检测浏览器语言
3. **语言存储**：使用 localStorage 存储用户语言选择
4. **动态内容**：JavaScript 动态生成的内容需要手动调用 `i18n.t()` 进行翻译
5. **扩展性**：添加新语言只需在 `translations` 对象中添加对应的语言配置

## 下一步

1. 更新 login.html 添加语言切换和 data-i18n 属性
2. 更新 index.html 添加语言切换和 data-i18n 属性
3. 更新 dashboard.html 添加语言切换和 data-i18n 属性
4. 更新相关 JavaScript 文件使用 i18n.t() 方法
5. 测试所有页面的双语切换功能
