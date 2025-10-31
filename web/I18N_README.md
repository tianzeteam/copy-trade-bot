# Copy Trade Bot 国际化 (i18n) 实现指南

## 📋 概述

为 Copy Trade Bot Web 管理界面实现了完整的中英文双语支持系统。用户可以在界面上自由切换语言，选择会被保存并在下次访问时自动应用。

## ✅ 已完成的工作

### 1. 核心文件
- ✅ **i18n.js** - 国际化核心框架
  - 支持中文和英文两种语言
  - 自动检测和保存用户语言偏好
  - 提供简单的 API 用于翻译文本
  - 支持动态更新页面内容

### 2. 示例文件
- ✅ **login_i18n.html** - 登录页面国际化示例
  - 完整的双语支持
  - 优雅的语言切换按钮
  - 所有文本元素都已标记翻译键

### 3. 文档
- ✅ **I18N_IMPLEMENTATION.md** - 详细实现方案
- ✅ **I18N_README.md** - 使用指南（本文档）

## 🚀 快速开始

### 步骤 1: 引入 i18n.js

在 HTML 页面底部引入国际化脚本：

```html
<script src="/static/js/i18n.js"></script>
```

### 步骤 2: 标记需要翻译的元素

在 HTML 元素上添加 `data-i18n` 属性：

```html
<!-- 文本内容 -->
<h1 data-i18n="app.title">Copy Trade Bot</h1>

<!-- 按钮 -->
<button data-i18n="app.save">保存</button>

<!-- 输入框 placeholder -->
<input type="text" data-i18n-placeholder="wallet.address" placeholder="钱包地址">

<!-- 元素 title 属性 -->
<button data-i18n-title="app.delete" title="删除">
    <i class="bi bi-trash"></i>
</button>
```

### 步骤 3: 添加语言切换按钮

```html
<div class="language-switcher">
    <button data-lang-switch="zh" class="active">中文</button>
    <button data-lang-switch="en">English</button>
</div>
```

### 步骤 4: 在 JavaScript 中使用

```javascript
// 获取翻译文本
const title = i18n.t('app.title');

// 显示提示消息
alert(i18n.t('msg.operation_success'));

// 设置语言
i18n.setLang('en');

// 获取当前语言
const currentLang = i18n.getCurrentLang();
```

## 📚 翻译键参考

### 通用
- `app.title` - 应用标题
- `app.save` - 保存
- `app.cancel` - 取消
- `app.delete` - 删除
- `app.edit` - 编辑
- `app.add` - 添加
- `app.update` - 更新
- `app.confirm` - 确认
- `app.close` - 关闭

### 登录页面
- `login.title` - 登录
- `login.username` - 用户名
- `login.password` - 密码
- `login.remember` - 记住我
- `login.submit` - 登录按钮
- `login.error` - 登录错误提示

### 钱包管理
- `wallet.title` - 钱包列表
- `wallet.add` - 添加钱包
- `wallet.select` - 选择监控钱包
- `wallet.name` - 钱包名称
- `wallet.address` - 钱包地址
- `wallet.type` - 钱包类型
- `wallet.enabled` - 启用此钱包监控
- `wallet.delete` - 删除钱包
- `wallet.save` - 保存更改

### 交易参数
- `trade.title` - 交易开关设置
- `trade.take_profit` - 止盈百分比
- `trade.stop_loss` - 止损百分比
- `trade.holding_time` - 持仓时间
- `trade.buy_amount` - 买入金额
- `trade.slippage` - 滑点容忍度
- `trade.priority_fee` - 优先费用

### 消息提示
- `msg.confirm_delete` - 确定要删除吗？
- `msg.operation_success` - 操作成功
- `msg.operation_failed` - 操作失败
- `msg.network_error` - 网络错误

完整的翻译键列表请查看 `i18n.js` 文件中的 `translations` 对象。

## 🎨 样式建议

### 语言切换按钮样式

```css
.language-switcher {
    position: fixed;
    top: 20px;
    right: 20px;
    z-index: 1000;
    display: flex;
    gap: 5px;
}

.language-switcher button {
    background-color: rgba(255, 255, 255, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.3);
    color: white;
    padding: 5px 12px;
    border-radius: 6px;
    font-size: 13px;
    cursor: pointer;
    transition: all 0.3s ease;
}

.language-switcher button:hover {
    background-color: rgba(255, 255, 255, 0.3);
}

.language-switcher button.active {
    background-color: white;
    color: #2196f3;
    font-weight: 600;
}
```

## 🔧 高级用法

### 监听语言变更事件

```javascript
window.addEventListener('languageChanged', (event) => {
    console.log('Language changed to:', event.detail.lang);
    // 重新加载数据或更新界面
    loadData();
});
```

### 动态更新语言按钮状态

```javascript
function updateLanguageButtons() {
    const currentLang = i18n.getCurrentLang();
    document.querySelectorAll('[data-lang-switch]').forEach(button => {
        const lang = button.getAttribute('data-lang-switch');
        if (lang === currentLang) {
            button.classList.add('active');
        } else {
            button.classList.remove('active');
        }
    });
}

// 监听语言变更
window.addEventListener('languageChanged', updateLanguageButtons);

// 页面加载时初始化
document.addEventListener('DOMContentLoaded', updateLanguageButtons);
```

### 在 AJAX 请求中使用

```javascript
async function saveWallet() {
    try {
        const response = await fetch('/api/wallet', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(walletData)
        });
        
        if (response.ok) {
            alert(i18n.t('wallet.add_success'));
        } else {
            alert(i18n.t('msg.operation_failed'));
        }
    } catch (error) {
        alert(i18n.t('msg.network_error'));
    }
}
```

### 在确认对话框中使用

```javascript
function deleteWallet(walletId) {
    if (confirm(i18n.t('wallet.delete_confirm'))) {
        // 执行删除操作
        performDelete(walletId);
    }
}
```

## 📝 待完成任务

### 需要更新的页面

1. **login.html** - 登录页面
   - 替换为 `login_i18n.html` 或手动添加 i18n 支持
   - 添加语言切换按钮
   - 标记所有文本元素

2. **index.html** - 钱包管理页面
   - 添加语言切换按钮
   - 标记所有静态文本
   - 更新 JavaScript 代码使用 i18n.t()

3. **dashboard.html** - 仪表板页面
   - 添加语言切换按钮
   - 标记所有静态文本
   - 更新图表和统计信息的标签

### 需要更新的 JavaScript 文件

检查以下文件中的硬编码文本：
- `/static/js/*.js`
- 内联的 `<script>` 标签

将所有用户可见的文本替换为 `i18n.t()` 调用。

## 🧪 测试清单

- [ ] 登录页面语言切换
- [ ] 钱包管理页面语言切换
- [ ] 仪表板页面语言切换
- [ ] 语言选择持久化（刷新页面后保持）
- [ ] 动态生成内容的翻译
- [ ] 表单验证消息翻译
- [ ] Toast 通知消息翻译
- [ ] 确认对话框翻译
- [ ] 所有按钮文本翻译
- [ ] 所有标签和提示文本翻译

## 🌍 添加新语言

如果需要添加其他语言（如日语、韩语等），只需在 `i18n.js` 的 `translations` 对象中添加新的语言配置：

```javascript
const translations = {
    zh: { /* 中文翻译 */ },
    en: { /* 英文翻译 */ },
    ja: { /* 日语翻译 */ },
    ko: { /* 韩语翻译 */ }
};
```

然后在语言切换按钮中添加对应选项：

```html
<button data-lang-switch="ja">日本語</button>
<button data-lang-switch="ko">한국어</button>
```

## 💡 最佳实践

1. **一致性**：使用统一的命名规范，如 `模块.功能` (e.g., `wallet.add`, `trade.title`)

2. **完整性**：确保所有用户可见的文本都有翻译键

3. **可维护性**：将相关的翻译键分组，便于管理

4. **测试**：在添加新功能时，同时添加对应的翻译键

5. **文档**：更新翻译键时，同步更新文档

## 🔗 相关文件

- `/web/static/js/i18n.js` - 国际化核心框架
- `/web/static/login_i18n.html` - 登录页面示例
- `/web/I18N_IMPLEMENTATION.md` - 详细实现方案
- `/web/I18N_README.md` - 本文档

## 📞 支持

如有问题或建议，请联系开发团队。

---

**版本**: 1.0.0  
**最后更新**: 2025-01-31
