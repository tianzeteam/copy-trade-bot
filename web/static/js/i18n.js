// 国际化配置文件
const translations = {
    zh: {
        // 通用
        'app.title': 'Copy Trade Bot - 聪明钱包管理',
        'app.save': '保存',
        'app.cancel': '取消',
        'app.delete': '删除',
        'app.edit': '编辑',
        'app.add': '添加',
        'app.update': '更新',
        'app.confirm': '确认',
        'app.close': '关闭',
        'app.success': '成功',
        'app.error': '错误',
        'app.warning': '警告',
        'app.loading': '加载中...',
        
        // 导航
        'nav.dashboard': '仪表板',
        'nav.wallets': '聪明钱包管理',
        'nav.private_keys': '交易钱包管理',
        'nav.param_scheme': '参数模板',
        'nav.settings': '交易设置',
        'nav.global_sell': '卖出全局设置',
        'nav.blacklist': '黑名单管理',
        'nav.logout': '退出',
        'nav.language': '语言',
        'nav.restart': '重启服务',
        
        // 通用操作
        'app.export': '导出',
        'app.yes': '是',
        'app.no': '否',
        
        // 登录页面
        'login.title': '登录',
        'login.username': '用户名',
        'login.password': '密码',
        'login.remember': '记住我',
        'login.submit': '登录',
        'login.error': '用户名或密码错误',
        
        // 钱包管理
        'wallet.title': '钱包列表',
        'wallet.add': '添加钱包',
        'wallet.select': '选择监控钱包',
        'wallet.name': '钱包名称',
        'wallet.address': '钱包地址',
        'wallet.type': '钱包类型',
        'wallet.enabled': '启用此钱包监控',
        'wallet.delete': '删除钱包',
        'wallet.save': '保存更改',
        'wallet.no_wallet': '没有可用的钱包。请添加一个新钱包。',
        'wallet.add_modal_title': '添加新钱包',
        'wallet.add_success': '钱包添加成功',
        'wallet.update_success': '钱包更新成功',
        'wallet.delete_success': '钱包删除成功',
        'wallet.delete_confirm': '确定要删除这个钱包吗？',
        
        // 交易参数
        'trade.title': '交易开关设置',
        'trade.take_profit': '止盈百分比',
        'trade.stop_loss': '止损百分比',
        'trade.holding_time': '持仓时间',
        'trade.buy_amount': '买入金额',
        'trade.slippage': '滑点容忍度',
        'trade.priority_fee': '优先费用',
        'trade.enabled': '启用交易',
        'trade.auto_sell': '自动卖出',
        'trade.seconds': '秒',
        'trade.sol': 'SOL',
        'trade.percent': '%',
        'trade.pump': 'Pump 交易',
        'trade.pump_amm': 'Pump AMM 交易',
        'trade.raydium': 'Raydium 交易',
        'trade.launchpad': 'Launchpad 交易',
        'trade.pool_size': '池子大小设置',
        'trade.min_pool': '最小池子大小',
        'trade.max_pool': '最大池子大小',
        'trade.trigger_amount': '触发买入数量设置',
        'trade.min_trigger': '最小触发买入数量',
        'trade.max_trigger': '最大触发买入数量',
        'trade.params': '交易参数设置',
        'trade.buy_slippage': '买入滑点',
        'trade.buy_priority': '优先级费用',
        'trade.buy_tip': 'Tip费用',
        'trade.sell_slippage': '卖出滑点',
        'trade.sell_priority': '优先级费用',
        'trade.sell_tip': 'Tip费用',
        'trade.sell_delay': '延迟卖出时间',
        'trade.save_settings': '保存交易设置',
        'trade.milliseconds': '毫秒',
        
        // 交易设置
        'settings.title': '交易设置',
        'settings.buy_params': '买入参数',
        'settings.sell_params': '卖出参数',
        'settings.risk_control': '风险控制',
        'settings.advanced': '高级设置',
        'settings.save_success': '设置保存成功',
        
        // 黑名单
        'blacklist.title': '黑名单管理',
        'blacklist.add': '添加到黑名单',
        'blacklist.address': '地址',
        'blacklist.reason': '原因',
        'blacklist.add_success': '已添加到黑名单',
        'blacklist.remove_success': '已从黑名单移除',
        
        // 参数方案
        'scheme.title': '参数方案',
        'scheme.name': '方案名称',
        'scheme.description': '方案描述',
        'scheme.create': '创建方案',
        'scheme.apply': '应用方案',
        'scheme.default': '默认方案',
        'scheme.custom': '自定义方案',
        
        // 统计信息
        'stats.total_trades': '总交易数',
        'stats.success_rate': '成功率',
        'stats.total_profit': '总收益',
        'stats.today_trades': '今日交易',
        
        // 消息提示
        'msg.confirm_delete': '确定要删除吗？',
        'msg.operation_success': '操作成功',
        'msg.operation_failed': '操作失败',
        'msg.please_select': '请选择',
        'msg.required_field': '此字段为必填项',
        'msg.invalid_address': '无效的地址',
        'msg.network_error': '网络错误，请稍后重试',
        'msg.success_toast': '操作成功！',
        'msg.error_toast': '操作失败！请检查输入并重试。',
        
        // 表单提示
        'form.example': '例如',
        'form.range': '范围',
        'form.pool_min_hint': '设置交易池最小大小阈值',
        'form.pool_max_hint': '设置交易池最大大小阈值',
        'form.trigger_min_hint': '设置触发交易的最小数量阈值',
        'form.trigger_max_hint': '设置触发交易的最大数量阈值',
        'form.trade_priority_hint': '交易优先级费用',
        'form.tip_hint': '给验证者的额外奖励',
        'form.buy_amount_hint': '单次交易金额',
        'form.sell_delay_hint': '卖出延迟时间(毫秒)',
        'form.valid_solana': '输入有效的Solana钱包地址',
        
        // 钱包类型
        'wallet.high_frequency': '高频交易',
        'wallet.low_frequency': '低频交易',
        'wallet.take_profit_label': '止盈',
        'wallet.stop_loss_label': '止损',
        'wallet.holding_time_label': '持仓时间',
        'wallet.no_note': '暂无备注信息',
        'wallet.copy_address': '复制地址',
        'wallet.config': '配置',
        'wallet.enable': '启用',
        'wallet.disable': '停用',
        'wallet.running': '运行',
        'wallet.stopped': '停用',
        'wallet.delete_wallet': '删除钱包',
        'wallet.delete_confirm': '确定要删除该钱包吗？此操作不可撤销。',
        'wallet.deleted': '钱包已删除',
        'wallet.delete_failed': '删除失败',
        'wallet.follow_sell': '同比例跟卖',
        'wallet.follow_sell_enabled': '已开启',
        'wallet.follow_sell_disabled': '已关闭',
        'wallet.param_template': '参数模板',
        'wallet.loading': '加载中...',
        'wallet.no_template': '未绑定模板',
        'wallet.unnamed': '未命名钱包',
        
        // 模态框
        'modal.add_wallet': '添加新钱包',
        'modal.cancel': '取消',
        'modal.add': '添加钱包',
        'modal.confirm_restart': '确定重启',
        'modal.restart_warning': '确定要重启服务吗？这将中断当前所有交易。',
        
        // 统计标签
        'stats.wallet_count': '个',
        'stats.running': '运行',
        'stats.stopped': '停用',
        'stats.risk_all': '风险: 全部',
        'stats.risk_high': '高风险',
        'stats.risk_medium': '中风险',
        'stats.risk_low': '低风险',
        'stats.risk_not_assessed': '未评估',
        
        // 视图
        'view.card': '卡片视图',
        'view.table': '表格视图',
        
        // 表格
        'table.address': '钱包地址',
        'table.note': '备注',
        'table.risk': '风险',
        'table.participation': '内/外参与',
        'table.enabled': '启用',
        'table.actions': '操作',
    },
    
    en: {
        // Common
        'app.title': 'Copy Trade Bot - Smart Wallet Management',
        'app.save': 'Save',
        'app.cancel': 'Cancel',
        'app.delete': 'Delete',
        'app.edit': 'Edit',
        'app.add': 'Add',
        'app.update': 'Update',
        'app.confirm': 'Confirm',
        'app.close': 'Close',
        'app.success': 'Success',
        'app.error': 'Error',
        'app.warning': 'Warning',
        'app.loading': 'Loading...',
        
        // Navigation
        'nav.dashboard': 'Dashboard',
        'nav.wallets': 'Smart Wallet Management',
        'nav.private_keys': 'Trading Wallet Management',
        'nav.param_scheme': 'Parameter Templates',
        'nav.settings': 'Trading Settings',
        'nav.global_sell': 'Global Sell Settings',
        'nav.blacklist': 'Blacklist Management',
        'nav.logout': 'Logout',
        'nav.language': 'Language',
        'nav.restart': 'Restart Service',
        
        // Common Actions
        'app.export': 'Export',
        'app.yes': 'Yes',
        'app.no': 'No',
        
        // Login Page
        'login.title': 'Login',
        'login.username': 'Username',
        'login.password': 'Password',
        'login.remember': 'Remember Me',
        'login.submit': 'Login',
        'login.error': 'Invalid username or password',
        
        // Wallet Management
        'wallet.title': 'Wallet List',
        'wallet.add': 'Add Wallet',
        'wallet.select': 'Select Monitoring Wallet',
        'wallet.name': 'Wallet Name',
        'wallet.address': 'Wallet Address',
        'wallet.type': 'Wallet Type',
        'wallet.enabled': 'Enable wallet monitoring',
        'wallet.delete': 'Delete Wallet',
        'wallet.save': 'Save Changes',
        'wallet.no_wallet': 'No wallets available. Please add a new wallet.',
        'wallet.add_modal_title': 'Add New Wallet',
        'wallet.add_success': 'Wallet added successfully',
        'wallet.update_success': 'Wallet updated successfully',
        'wallet.delete_success': 'Wallet deleted successfully',
        'wallet.delete_confirm': 'Are you sure you want to delete this wallet?',
        
        // Trading Parameters
        'trade.title': 'Trading Switch Settings',
        'trade.take_profit': 'Take Profit %',
        'trade.stop_loss': 'Stop Loss %',
        'trade.holding_time': 'Holding Time',
        'trade.buy_amount': 'Buy Amount',
        'trade.slippage': 'Slippage Tolerance',
        'trade.priority_fee': 'Priority Fee',
        'trade.enabled': 'Enable Trading',
        'trade.auto_sell': 'Auto Sell',
        'trade.seconds': 'seconds',
        'trade.sol': 'SOL',
        'trade.percent': '%',
        'trade.pump': 'Pump Trading',
        'trade.pump_amm': 'Pump AMM Trading',
        'trade.raydium': 'Raydium Trading',
        'trade.launchpad': 'Launchpad Trading',
        'trade.pool_size': 'Pool Size Settings',
        'trade.min_pool': 'Min Pool Size',
        'trade.max_pool': 'Max Pool Size',
        'trade.trigger_amount': 'Trigger Buy Amount Settings',
        'trade.min_trigger': 'Min Trigger Buy Amount',
        'trade.max_trigger': 'Max Trigger Buy Amount',
        'trade.params': 'Trading Parameters',
        'trade.buy_slippage': 'Buy Slippage',
        'trade.buy_priority': 'Priority Fee',
        'trade.buy_tip': 'Tip Fee',
        'trade.sell_slippage': 'Sell Slippage',
        'trade.sell_priority': 'Priority Fee',
        'trade.sell_tip': 'Tip Fee',
        'trade.sell_delay': 'Sell Delay Time',
        'trade.save_settings': 'Save Trade Settings',
        'trade.milliseconds': 'ms',
        
        // Trading Settings
        'settings.title': 'Trading Settings',
        'settings.buy_params': 'Buy Parameters',
        'settings.sell_params': 'Sell Parameters',
        'settings.risk_control': 'Risk Control',
        'settings.advanced': 'Advanced Settings',
        'settings.save_success': 'Settings saved successfully',
        
        // Blacklist
        'blacklist.title': 'Blacklist Management',
        'blacklist.add': 'Add to Blacklist',
        'blacklist.address': 'Address',
        'blacklist.reason': 'Reason',
        'blacklist.add_success': 'Added to blacklist',
        'blacklist.remove_success': 'Removed from blacklist',
        
        // Parameter Schemes
        'scheme.title': 'Parameter Schemes',
        'scheme.name': 'Scheme Name',
        'scheme.description': 'Description',
        'scheme.create': 'Create Scheme',
        'scheme.apply': 'Apply Scheme',
        'scheme.default': 'Default Scheme',
        'scheme.custom': 'Custom Scheme',
        
        // Statistics
        'stats.total_trades': 'Total Trades',
        'stats.success_rate': 'Success Rate',
        'stats.total_profit': 'Total Profit',
        'stats.today_trades': 'Today\'s Trades',
        
        // Messages
        'msg.confirm_delete': 'Are you sure you want to delete?',
        'msg.operation_success': 'Operation successful',
        'msg.operation_failed': 'Operation failed',
        'msg.please_select': 'Please select',
        'msg.required_field': 'This field is required',
        'msg.invalid_address': 'Invalid address',
        'msg.network_error': 'Network error, please try again later',
        'msg.success_toast': 'Operation successful!',
        'msg.error_toast': 'Operation failed! Please check your input and try again.',
        
        // Form Hints
        'form.example': 'e.g.',
        'form.range': 'Range',
        'form.pool_min_hint': 'Set minimum pool size threshold',
        'form.pool_max_hint': 'Set maximum pool size threshold',
        'form.trigger_min_hint': 'Set minimum trigger amount threshold',
        'form.trigger_max_hint': 'Set maximum trigger amount threshold',
        'form.trade_priority_hint': 'Transaction priority fee',
        'form.tip_hint': 'Extra reward for validators',
        'form.buy_amount_hint': 'Amount per transaction',
        'form.sell_delay_hint': 'Sell delay time (milliseconds)',
        'form.valid_solana': 'Enter a valid Solana wallet address',
        
        // Wallet Types
        'wallet.high_frequency': 'High Frequency Trading',
        'wallet.low_frequency': 'Low Frequency Trading',
        'wallet.take_profit_label': 'Take Profit',
        'wallet.stop_loss_label': 'Stop Loss',
        'wallet.holding_time_label': 'Holding Time',
        'wallet.no_note': 'No notes available',
        'wallet.copy_address': 'Copy Address',
        'wallet.config': 'Config',
        'wallet.enable': 'Enable',
        'wallet.disable': 'Disable',
        'wallet.running': 'Running',
        'wallet.stopped': 'Stopped',
        'wallet.delete_wallet': 'Delete Wallet',
        'wallet.delete_confirm': 'Are you sure you want to delete this wallet? This action cannot be undone.',
        'wallet.deleted': 'Wallet deleted',
        'wallet.delete_failed': 'Delete failed',
        'wallet.follow_sell': 'Proportional Follow Sell',
        'wallet.follow_sell_enabled': 'Enabled',
        'wallet.follow_sell_disabled': 'Disabled',
        'wallet.param_template': 'Parameter Template',
        'wallet.loading': 'Loading...',
        'wallet.no_template': 'No template bound',
        'wallet.unnamed': 'Unnamed Wallet',
        
        // Modals
        'modal.add_wallet': 'Add New Wallet',
        'modal.cancel': 'Cancel',
        'modal.add': 'Add Wallet',
        'modal.confirm_restart': 'Confirm Restart',
        'modal.restart_warning': 'Are you sure you want to restart the service? This will interrupt all current trades.',
        
        // Statistics
        'stats.wallet_count': '',
        'stats.running': 'Running',
        'stats.stopped': 'Stopped',
        'stats.risk_all': 'Risk: All',
        'stats.risk_high': 'High Risk',
        'stats.risk_medium': 'Medium Risk',
        'stats.risk_low': 'Low Risk',
        'stats.risk_not_assessed': 'Not Assessed',
        
        // Views
        'view.card': 'Card View',
        'view.table': 'Table View',
        
        // Table
        'table.address': 'Wallet Address',
        'table.note': 'Note',
        'table.risk': 'Risk',
        'table.participation': 'Internal/External',
        'table.enabled': 'Enabled',
        'table.actions': 'Actions',
    }
};

// 国际化管理类
class I18n {
    constructor() {
        this.currentLang = localStorage.getItem('language') || 'zh';
        this.translations = translations;
    }
    
    // 获取当前语言
    getCurrentLang() {
        return this.currentLang;
    }
    
    // 设置语言
    setLang(lang) {
        if (this.translations[lang]) {
            this.currentLang = lang;
            localStorage.setItem('language', lang);
            this.updatePageText();
            // 触发语言变更事件
            window.dispatchEvent(new CustomEvent('languageChanged', { detail: { lang } }));
        }
    }
    
    // 获取翻译文本
    t(key, defaultValue = '') {
        const translation = this.translations[this.currentLang];
        return translation && translation[key] ? translation[key] : (defaultValue || key);
    }
    
    // 更新页面所有文本
    updatePageText() {
        // 更新所有带 data-i18n 属性的元素
        document.querySelectorAll('[data-i18n]').forEach(element => {
            const key = element.getAttribute('data-i18n');
            const text = this.t(key);
            
            // 根据元素类型更新文本
            if (element.tagName === 'INPUT' || element.tagName === 'TEXTAREA') {
                if (element.type === 'button' || element.type === 'submit') {
                    element.value = text;
                } else {
                    element.placeholder = text;
                }
            } else {
                element.textContent = text;
            }
        });
        
        // 更新 title 属性
        document.querySelectorAll('[data-i18n-title]').forEach(element => {
            const key = element.getAttribute('data-i18n-title');
            element.title = this.t(key);
        });
        
        // 更新 placeholder 属性
        document.querySelectorAll('[data-i18n-placeholder]').forEach(element => {
            const key = element.getAttribute('data-i18n-placeholder');
            element.placeholder = this.t(key);
        });
        
        // 更新页面标题
        const titleElement = document.querySelector('title');
        if (titleElement && titleElement.hasAttribute('data-i18n')) {
            titleElement.textContent = this.t(titleElement.getAttribute('data-i18n'));
        }
    }
    
    // 初始化
    init() {
        this.updatePageText();
        
        // 添加语言切换按钮事件监听
        document.querySelectorAll('[data-lang-switch]').forEach(button => {
            button.addEventListener('click', (e) => {
                const lang = e.target.getAttribute('data-lang-switch');
                this.setLang(lang);
            });
        });
    }
}

// 创建全局实例
const i18n = new I18n();

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', () => {
    i18n.init();
});

// 导出供其他脚本使用
window.i18n = i18n;
