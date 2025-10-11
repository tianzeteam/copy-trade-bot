// 全局变量
let currentWalletAddress = '';
let authToken = '';
// 新增：钱包管理视图与缓存
let walletViewMode = 'cards'; // 'cards' | 'table'
let walletsCache = [];
let statusFilter = ''; // '' | 'active' | 'inactive'

// 配置常量
const CONFIG = {
    API_ENDPOINTS: {
        WALLETS: '/api/wallets',
        PARAM_SCHEMES: '/api/param-schemes',
        TRADE_SETTINGS: '/api/trade-settings',
        GLOBAL_SELL: '/api/redis-config',
        PRIVATE_KEYS: '/api/wallets/private-keys',
        BLACKLIST: '/api/blacklist'
    },
    SECTIONS: ['walletManagement', 'paramScheme', 'tradeSettings', 'globalSellSettings', 'blacklistManagement', 'privateKeyManagement']
};

// 页面加载完成后执行
document.addEventListener('DOMContentLoaded', function() {
    // 检查是否已登录
    authToken = localStorage.getItem('auth_token');
    if (!authToken) {
        // 未登录，重定向到登录页面
        window.location.href = '/login';
        return;
    }

    // 显示当前用户
    const username = localStorage.getItem('username');
    if (username) {
        document.getElementById('currentUser').textContent = `当前用户: ${username}`;
    }

    // 初始化页面
    initializePage();
    
    // 注册事件处理器
    registerEventHandlers();
});

// 初始化页面
function initializePage() {
    // 默认显示钱包管理部分
    showSection('walletManagement');
    
    // 加载钱包列表
    loadWallets();
    
    // 加载交易设置
    loadTradeSettings();
    
    // 加载参数模板列表
    loadParamSchemes();
    
    // 加载私钥钱包列表
    loadPrivateKeyWallets();
}

// 注册事件处理器
function registerEventHandlers() {
    // 导航链接点击事件 - 使用事件委托优化
    const navLinks = {
        'walletManagementLink': 'walletManagement',
        'paramSchemeLink': 'paramScheme', 
        'tradeSettingsLink': 'tradeSettings',
        'globalSellSettingsLink': 'globalSellSettings',
        'blacklistManagementLink': 'blacklistManagement',
        'privateKeyManagementLink': 'privateKeyManagement'
    };
    
    Object.entries(navLinks).forEach(([linkId, sectionName]) => {
        const element = document.getElementById(linkId);
        if (element) {
            element.addEventListener('click', (e) => {
                e.preventDefault();
                showSection(sectionName);
            });
        }
    });
    
    // 退出登录按钮
    document.getElementById('logoutBtn').addEventListener('click', function() {
        logout();
    });
    
    // 钱包相关事件处理 - 增加空值检查
    const walletSelect = document.getElementById('walletSelect');
    if (walletSelect) {
        walletSelect.addEventListener('change', function() {
            const selectedAddress = this.value;
            if (selectedAddress) {
                loadWalletDetails(selectedAddress);
            } else {
                hideWalletDetails();
            }
        });
    }
    
    // 钱包操作按钮事件 - 统一处理
    const walletButtons = {
        'updateWalletBtn': updateWallet,
        'deleteWalletBtn': deleteWallet,
        'saveNewWalletBtn': addNewWallet
    };
    
    Object.entries(walletButtons).forEach(([buttonId, handler]) => {
        const button = document.getElementById(buttonId);
        if (button) {
            button.addEventListener('click', handler);
        }
    });
    
    // 新增：卡片/表格视图切换
    setupViewToggle();
    
    // 新增：筛选功能
    setupFilters();
    
    // 新增：导出和配置功能
    setupWalletActions();
    
    // 私钥管理相关事件
    document.getElementById('privateKeyManagementLink').addEventListener('click', function(e) {
        e.preventDefault();
        showSection('privateKeyManagement');
    });
    
    document.getElementById('confirmImportPrivateKeyBtn').addEventListener('click', function() {
        importPrivateKeyWallet();
    });
    
    // 保存交易设置按钮
    document.getElementById('saveTradeSettingsBtn').addEventListener('click', function() {
        saveTradeSettings();
    });
    
    // 保存全局卖出设置按钮
    document.getElementById('saveGlobalSellSettingsBtn').addEventListener('click', function() {
        saveGlobalSellSettings();
    });
    
    // 参数模板选择变更事件
    document.getElementById('schemeSelect').addEventListener('change', function() {
        const selectedSchemeId = this.value;
        if (selectedSchemeId) {
            loadSchemeDetails(selectedSchemeId);
        } else {
            hideSchemeDetails();
        }
    });
    
    // 保存参数模板设置按钮
    document.getElementById('updateSchemeBtn').addEventListener('click', function() {
        updateScheme();
    });
    
    // 删除参数模板按钮
    document.getElementById('deleteSchemeBtn').addEventListener('click', function() {
        deleteScheme();
    });
    
    // 添加新参数模板按钮
    document.getElementById('saveNewSchemeBtn').addEventListener('click', function() {
        addNewScheme();
    });
    
    // 滑点输入验证 - 买入滑点
    const buySlippageInput = document.getElementById('buySlippage');
    if (buySlippageInput) {
        buySlippageInput.addEventListener('input', function() {
            enforceSlippageRange(this);
        });
        buySlippageInput.addEventListener('blur', function() {
            enforceSlippageRange(this, true);
        });
    }
    
    // 滑点输入验证 - 卖出滑点
    const sellSlippageInput = document.getElementById('sellSlippage');
    if (sellSlippageInput) {
        sellSlippageInput.addEventListener('input', function() {
            enforceSlippageRange(this);
        });
        sellSlippageInput.addEventListener('blur', function() {
            enforceSlippageRange(this, true);
        });
    }
}

// 显示指定部分 - 优化性能和代码复用
function showSection(sectionName) {
    // 隐藏所有部分
    CONFIG.SECTIONS.forEach(section => {
        const element = document.getElementById(section + 'Section');
        if (element) {
            element.style.display = 'none';
        }
    });
    
    // 移除所有导航链接的激活状态
    document.querySelectorAll('.nav-link').forEach(link => {
        link.classList.remove('active');
    });
    
    // 显示指定部分
    const targetSection = document.getElementById(sectionName + 'Section');
    if (targetSection) {
        targetSection.style.display = 'block';
    }
    
    // 激活对应的导航链接
    const targetLink = document.getElementById(sectionName + 'Link');
    if (targetLink) {
        targetLink.classList.add('active');
    }
    
    // 特殊处理某些页面的数据加载
    const sectionHandlers = {
        'globalSellSettings': loadGlobalSellSettings,
        'blacklistManagement': loadBlacklist,
        'walletManagement': () => {
            if (walletsCache.length === 0) loadWallets();
        }
    };
    
    if (sectionHandlers[sectionName]) {
        sectionHandlers[sectionName]();
    }
}

// 新增辅助函数
function setupViewToggle() {
    const viewCardsBtn = document.getElementById('walletViewCardsBtn');
    const viewTableBtn = document.getElementById('walletViewTableBtn');
    if (viewCardsBtn && viewTableBtn) {
        viewCardsBtn.addEventListener('click', () => {
            walletViewMode = 'cards';
            viewCardsBtn.classList.add('active');
            viewTableBtn.classList.remove('active');
            document.getElementById('walletCards').classList.remove('d-none');
            document.getElementById('walletTableWrapper').classList.add('d-none');
            renderWallets();
        });
        viewTableBtn.addEventListener('click', () => {
            walletViewMode = 'table';
            viewTableBtn.classList.add('active');
            viewCardsBtn.classList.remove('active');
            document.getElementById('walletCards').classList.add('d-none');
            document.getElementById('walletTableWrapper').classList.remove('d-none');
            renderWallets();
        });
    }
}

function setupFilters() {
    const riskSelect = document.getElementById('riskFilterSelect');
    if (riskSelect) riskSelect.addEventListener('change', renderWallets);
    
    // 添加状态徽章点击事件
    const countBadge = document.getElementById('walletCountBadge');
    const activeBadge = document.getElementById('walletActiveBadge');
    const inactiveBadge = document.getElementById('walletInactiveBadge');
    
    // 点击总数徽章显示所有钱包
    if (countBadge) {
        countBadge.addEventListener('click', () => {
            statusFilter = '';
            if (activeBadge) activeBadge.classList.remove('badge-filter-active');
            if (inactiveBadge) inactiveBadge.classList.remove('badge-filter-active');
            renderWallets();
        });
    }
    
    if (activeBadge) {
        activeBadge.addEventListener('click', () => {
            // 激活筛选运行中的钱包
            statusFilter = 'active';
            activeBadge.classList.add('badge-filter-active');
            if (inactiveBadge) inactiveBadge.classList.remove('badge-filter-active');
            renderWallets();
        });
    }
    
    if (inactiveBadge) {
        inactiveBadge.addEventListener('click', () => {
            // 激活筛选停用的钱包
            statusFilter = 'inactive';
            inactiveBadge.classList.add('badge-filter-active');
            if (activeBadge) activeBadge.classList.remove('badge-filter-active');
            renderWallets();
        });
    }
}

function setupWalletActions() {
    const exportBtn = document.getElementById('exportWalletsBtn');
    if (exportBtn) exportBtn.addEventListener('click', exportWalletsAsJSON);
    const saveCfgBtn = document.getElementById('saveWalletConfigBtn');
    if (saveCfgBtn) saveCfgBtn.addEventListener('click', saveWalletConfig);
}

// API请求封装 - 减少重复代码
async function apiRequest(url, options = {}) {
    const defaultOptions = {
        headers: {
            'Authorization': `Bearer ${authToken}`,
            'Content-Type': 'application/json'
        }
    };
    
    const config = { ...defaultOptions, ...options };
    if (options.headers) {
        config.headers = { ...defaultOptions.headers, ...options.headers };
    }
    
    try {
        const response = await fetch(url, config);
        if (response.status === 401) {
            logout();
            return null;
        }
        return await response.json();
    } catch (error) {
        console.error('API请求失败:', error);
        throw error;
    }
}

// 加载钱包列表 - 重构使用新的API封装
async function loadWallets() {
    try {
        const data = await apiRequest(CONFIG.API_ENDPOINTS.WALLETS);
        if (!data) return;
        
        // 缓存与统计
        walletsCache = Array.isArray(data) ? data : [];
        updateWalletStatistics();
        
        // 兼容旧的下拉选择器
        updateWalletSelect();
        
        // 渲染新的卡片/表格视图
        renderWallets();
    } catch (error) {
        console.error('加载钱包列表失败:', error);
        showError('加载钱包列表失败，请刷新页面重试');
    }
}

// 更新钱包统计信息
function updateWalletStatistics() {
    const total = walletsCache.length;
    const actives = walletsCache.filter(w => !!w.enabled).length;
    const inactives = total - actives;
    
    const updateBadge = (id, text) => {
        const el = document.getElementById(id);
        if (el) el.textContent = text;
    };
    
    updateBadge('walletCountBadge', `${total} 个`);
    updateBadge('walletActiveBadge', `运行 ${actives}`);
    updateBadge('walletInactiveBadge', `停用 ${inactives}`);
}

// 更新钱包下拉选择器
function updateWalletSelect() {
    const walletSelect = document.getElementById('walletSelect');
    if (!walletSelect) return;
    
    walletSelect.innerHTML = '<option value="">选择钱包...</option>';
    walletsCache.forEach(wallet => {
        const option = document.createElement('option');
        option.value = wallet.address;
        option.textContent = `${wallet.name || '未命名'} (${wallet.address.substring(0, 8)}...)`;
        walletSelect.appendChild(option);
    });
}

// 渲染钱包视图
function renderWallets() {
    const noAlert = document.getElementById('noWalletAlert');
    const risk = document.getElementById('riskFilterSelect')?.value || '';

    let filteredList = walletsCache.slice();
    if (risk) filteredList = filteredList.filter(w => (w.risk || '') === risk);
    if (statusFilter) filteredList = filteredList.filter(w => statusFilter === 'active' ? !!w.enabled : !w.enabled);

    // 排序：运行状态的钱包显示在前面，停用状态的显示在后面
    filteredList.sort((a, b) => {
        if (a.enabled === b.enabled) return 0;
        return a.enabled ? -1 : 1;
    });

    // 显示/隐藏无钱包提示
    if (noAlert) {
        noAlert.classList.toggle('d-none', filteredList.length > 0);
    }

    if (walletViewMode === 'cards') {
        renderWalletCards(filteredList);
    } else {
        renderWalletTable(filteredList);
    }
}

// 渲染钱包卡片
function renderWalletCards(wallets) {
    const grid = document.getElementById('walletCards');
    if (!grid) return;
    
    grid.innerHTML = '';
    wallets.forEach((wallet, index) => {
        const col = createWalletCard(wallet, index);
        grid.appendChild(col);
    });
}

// 渲染钱包表格
function renderWalletTable(wallets) {
    const tbody = document.querySelector('#walletTable tbody');
    if (!tbody) return;
    
    tbody.innerHTML = '';
    wallets.forEach((wallet, index) => {
        const row = createWalletRow(wallet, index);
        tbody.appendChild(row);
    });
}

// 创建钱包卡片
function createWalletCard(wallet, index) {
    const col = document.createElement('div');
    col.className = 'col';
    
    const riskBadge = getRiskBadge(wallet.risk);
    const statusBadge = wallet.enabled ? 
        '<span class="badge bg-success">运行</span>' : 
        '<span class="badge bg-secondary">停用</span>';
    const note = escapeHtml(wallet.note || '');

    col.innerHTML = `
        <div class="card h-100 overflow-hidden" style="aspect-ratio: 411/520; min-height: 160px; border-radius: 12px; transition: all 0.3s ease; ${!wallet.enabled ? 'border-color: #d1d5db !important;' : ''}">
            <div class="card-body d-flex flex-column p-0 position-relative" style="background: ${wallet.enabled ? 'linear-gradient(135deg, #ffffff 0%, #f8fafc 100%)' : 'linear-gradient(135deg, #f3f4f6 0%, #e5e7eb 100%)'};">
                <!-- 顶部装饰条 -->
                <div class="w-100 ${wallet.enabled ? 'shimmer-effect' : ''}" style="height: 6px; background: ${wallet.enabled ? 'linear-gradient(90deg, #3b82f6 0%, #06b6d4 50%, #10b981 100%)' : 'linear-gradient(90deg, #e5e7eb 0%, #d1d5db 50%, #e5e7eb 100%)'};"></div>

                <!-- 主要内容区 -->
                <div class="p-3 flex-grow-1 d-flex flex-column">
                    <!-- 头部信息 -->
                    <div class="mb-3">
                        <div class="d-flex align-items-center gap-2 mb-2">
                            <span class="badge bg-light text-dark border" style="font-size: 0.85rem; font-weight: 600;">#${index + 1}</span>
                            <span class="text-dark fw-semibold" style="font-size: 1rem;">${escapeHtml(wallet.name || '未命名钱包')}</span>
                            <button class="btn btn-sm btn-light border-0 text-muted" onclick="copyWalletAddress('${wallet.address}')" title="复制地址" style="padding: 3px 8px; border-radius: 6px; background: rgba(248,250,252,0.8); transition: all 0.2s ease;">
                                <i class="bi bi-copy" style="font-size: 0.9rem;"></i>
                            </button>
                        </div>
                        <div class="d-flex gap-1 mb-3">
                            ${riskBadge}
                            ${statusBadge}
                        </div>
                        <div class="text-muted mb-3" style="font-size: 0.95rem; font-family: 'SF Mono', 'Monaco', 'Cascadia Code', monospace; letter-spacing: 0.5px; word-break: break-all;">
                            ${wallet.address}
                        </div>
                        
                        <!-- 备注信息 -->
                        <div class="mb-3">
                            <div class="w-100 p-2 rounded" style="background: rgba(248,250,252,0.6); border: 1px solid rgba(226,232,240,0.8);">
                                <div class="text-muted small" style="line-height: 1.4; overflow: hidden; text-overflow: ellipsis; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; font-size: 0.9rem;">
                                    ${note || '暂无备注信息'}
                                </div>
                            </div>
                        </div>
                        
                        <!-- 跟卖状态显示 -->
                        <div class="mb-3">
                            <div class="d-flex align-items-center gap-2">
                                <span class="text-muted" style="font-size: 1.1rem;">同比例跟卖:</span>
                                <span class="badge ${wallet.follow_sell ? 'bg-success' : 'bg-secondary'}" style="font-size: 0.85rem;">
                                    ${wallet.follow_sell ? '已开启' : '已关闭'}
                                </span>
                            </div>
                        </div>
                        
                        <!-- 参数模板显示 -->
                        <div class="mb-4">
                            <div class="w-100 p-2 rounded" style="background: rgba(59, 130, 246, 0.05); border: 1px solid rgba(59, 130, 246, 0.15);">
                                <div class="d-flex align-items-center gap-2 mb-1">
                                    <i class="bi bi-gear-fill text-primary" style="font-size: 0.85rem;"></i>
                                    <span class="text-muted" style="font-size: 0.9rem; font-weight: 500;">参数模板:</span>
                                </div>
                                <div class="text-dark" style="font-size: 0.9rem; font-weight: 500;" id="paramScheme_${wallet.address}">
                                    ${wallet.param_scheme_id ? '加载中...' : '未绑定模板'}
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <!-- 操作按钮 -->
                    <div class="d-grid gap-2">
                        <div class="d-flex gap-2">
                            <button class="btn btn-primary btn-sm flex-fill" onclick="openWalletConfig('${wallet.address}')" style="border-radius: 6px; font-size: 0.9rem; font-weight: 500;">
                                <i class="bi bi-gear"></i> 配置
                            </button>
                            <button class="btn ${wallet.enabled ? 'btn-warning' : 'btn-success'} btn-sm flex-fill" 
                                    onclick="toggleWalletStatus('${wallet.address}')" style="border-radius: 6px; font-size: 0.9rem; font-weight: 500;">
                                <i class="bi bi-${wallet.enabled ? 'pause' : 'play'}"></i> ${wallet.enabled ? '停用' : '启用'}
                            </button>
                        </div>
                        <button class="btn btn-outline-danger btn-sm" onclick="deleteWalletByAddress('${wallet.address}')" style="border-radius: 6px; font-size: 0.9rem; font-weight: 500;">
                            <i class="bi bi-trash3"></i> 删除钱包
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `;
    
    // 如果钱包绑定了参数模板，异步加载模板名称
    if (wallet.param_scheme_id) {
        loadParamSchemeNameForCard(wallet.address, wallet.param_scheme_id);
    }
    return col;
}

// 创建钱包表格行
function createWalletRow(wallet, index) {
    const tr = document.createElement('tr');
    tr.innerHTML = `
        <td>${index + 1}</td>
        <td>${formatAddress(wallet.address)}</td>
        <td>${escapeHtml(wallet.note || '')}</td>
        <td>${getRiskText(wallet.risk)}</td>
        <td>${wallet.inner_count || 0} / ${wallet.outer_count || 0}</td>
        <td>${wallet.enabled ? '<span class="badge bg-success">是</span>' : '<span class="badge bg-secondary">否</span>'}</td>
        <td>
            <div class="btn-group btn-group-sm">
                <button class="btn btn-outline-primary" onclick="openWalletConfig('${wallet.address}')">配置</button>
                <button class="btn btn-outline-${wallet.enabled ? 'secondary' : 'success'}" 
                        onclick="toggleWalletStatus('${wallet.address}')">
                    ${wallet.enabled ? '停用' : '启用'}
                </button>
                <button class="btn btn-outline-danger" onclick="deleteWalletByAddress('${wallet.address}')">删除</button>
            </div>
        </td>`;
    return tr;
}

// 工具函数
function formatAddress(addr) {
    if (!addr) return '';
    return `${addr.slice(0, 4)}…${addr.slice(-4)}`;
}

function getRiskBadge(risk) {
    const badges = {
        'high': '<span class="badge bg-danger">高风险</span>',
        'medium': '<span class="badge bg-warning text-dark">中风险</span>',
        'low': '<span class="badge bg-success">低风险</span>'
    };
    return badges[risk] || '<span class="badge bg-light text-muted">未评估</span>';
}

function getRiskText(risk) {
    const texts = { 'high': '高', 'medium': '中', 'low': '低' };
    return texts[risk] || '未评估';
}

function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// 复制钱包地址到剪贴板
function copyWalletAddress(address) {
    navigator.clipboard.writeText(address).then(function() {
        // 显示成功提示
        showToast('地址已复制到剪贴板', 'success');
    }).catch(function(err) {
        // 降级处理：使用传统方法
        const textArea = document.createElement('textarea');
        textArea.value = address;
        document.body.appendChild(textArea);
        textArea.select();
        try {
            document.execCommand('copy');
            showToast('地址已复制到剪贴板', 'success');
        } catch (err) {
            showToast('复制失败，请手动复制', 'error');
        }
        document.body.removeChild(textArea);
    });
}

// 显示提示消息
function showToast(message, type = 'info') {
    // 创建toast元素
    const toast = document.createElement('div');
    toast.className = `alert alert-${type === 'success' ? 'success' : type === 'error' ? 'danger' : 'info'} alert-dismissible fade show position-fixed`;
    toast.style.cssText = 'top: 20px; right: 20px; z-index: 9999; min-width: 250px;';
    toast.innerHTML = `
        ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    
    document.body.appendChild(toast);
    
    // 3秒后自动移除
    setTimeout(() => {
        if (toast.parentNode) {
            toast.parentNode.removeChild(toast);
        }
    }, 3000);
}

// 切换跟卖状态
async function toggleFollowSell(address) {
    const wallet = walletsCache.find(w => w.address === address);
    if (!wallet) return;
    
    const payload = {
        name: wallet.name || '',
        risk: wallet.risk || '',
        note: wallet.note || '',
        type: wallet.type || 'high_frequency',
        take_profit_pct: wallet.take_profit_pct ?? 100,
        stop_loss_pct: wallet.stop_loss_pct ?? 10,
        enabled: !!wallet.enabled,
        follow_sell: !wallet.follow_sell,
        param_scheme_id: wallet.param_scheme_id || null
    };
    
    try {
        await apiRequest(`${CONFIG.API_ENDPOINTS.WALLETS}/${address}`, {
            method: 'PUT',
            body: JSON.stringify(payload)
        });
        
        // 更新缓存
        wallet.follow_sell = !wallet.follow_sell;
        showToast(`跟卖已${wallet.follow_sell ? '开启' : '关闭'}`, 'success');
        
        // 重新加载钱包列表以确保数据同步
        loadWallets();
    } catch (error) {
        // 恢复复选框状态
        const checkbox = document.getElementById(`followSell_${address}`);
        if (checkbox) {
            checkbox.checked = wallet.follow_sell;
        }
        showToast('操作失败: ' + error.message, 'error');
    }
}

// 钱包操作函数
function openWalletConfig(address) {
    const wallet = walletsCache.find(w => w.address === address);
    if (!wallet) return;
    
    // 填充配置模态框
    document.getElementById('cfgWalletAddress').value = wallet.address;
    document.getElementById('cfgWalletNote').value = wallet.note || '';
    document.getElementById('cfgWalletRisk').value = wallet.risk || '';
    document.getElementById('cfgWalletEnabled').checked = !!wallet.enabled;
    document.getElementById('cfgTakeProfitPct').value = wallet.take_profit_pct ?? 100;
    document.getElementById('cfgStopLossPct').value = wallet.stop_loss_pct ?? 10;
    document.getElementById('cfgWalletFollowSell').checked = !!wallet.follow_sell;
    
    // 加载参数模板选项
    loadParamSchemesForConfig(wallet.param_scheme_id);
    
    // 显示模态框
    const modal = new bootstrap.Modal(document.getElementById('walletConfigModal'));
    modal.show();
}

async function saveWalletConfig() {
    const address = document.getElementById('cfgWalletAddress').value;
    if (!address) return;
    
    // 从缓存中获取原有钱包信息以保持name字段
    const wallet = walletsCache.find(w => w.address === address);
    
    const payload = {
        address,
        name: wallet ? wallet.name : '', // 保持原有的name
        type: wallet ? wallet.type : 'high_frequency',
        take_profit_pct: parseFloat(document.getElementById('cfgTakeProfitPct').value),
        stop_loss_pct: parseFloat(document.getElementById('cfgStopLossPct').value),
        enabled: document.getElementById('cfgWalletEnabled').checked,
        follow_sell: document.getElementById('cfgWalletFollowSell').checked,
        param_scheme_id: document.getElementById('cfgWalletParamScheme').value || null,
        note: document.getElementById('cfgWalletNote').value || '',
        risk: document.getElementById('cfgWalletRisk').value || ''
    };
    
    try {
        const result = await apiRequest(`${CONFIG.API_ENDPOINTS.WALLETS}/${address}`, {
            method: 'PUT',
            body: JSON.stringify(payload)
        });
        
        if (result && (result.status === 'success' || result.message)) {
            showSuccess(result.message || '钱包配置已保存');
            bootstrap.Modal.getInstance(document.getElementById('walletConfigModal')).hide();
            loadWallets();
        } else {
            showError('保存失败: ' + (result?.error || '未知错误'));
        }
    } catch (error) {
        showError('保存失败: ' + error.message);
    }
}

async function toggleWalletStatus(address) {
    const wallet = walletsCache.find(w => w.address === address);
    if (!wallet) return;
    
    const payload = {
        address,
        name: wallet.name || '',
        risk: wallet.risk || '',
        note: wallet.note || '',
        type: wallet.type || 'high_frequency',
        take_profit_pct: wallet.take_profit_pct ?? 100,
        stop_loss_pct: wallet.stop_loss_pct ?? 10,
        enabled: !wallet.enabled,
        follow_sell: !!wallet.follow_sell,
        param_scheme_id: wallet.param_scheme_id || null
    };
    
    try {
        await apiRequest(`${CONFIG.API_ENDPOINTS.WALLETS}/${address}`, {
            method: 'PUT',
            body: JSON.stringify(payload)
        });
        loadWallets();
    } catch (error) {
        showError('操作失败: ' + error.message);
    }
}

async function deleteWalletByAddress(address) {
    if (!confirm('确定要删除该钱包吗？此操作不可撤销。')) return;
    
    try {
        const result = await apiRequest(`${CONFIG.API_ENDPOINTS.WALLETS}/${address}`, {
            method: 'DELETE'
        });
        
        if (result && (result.status === 'success' || result.message)) {
            showSuccess(result.message || '钱包已删除');
            loadWallets();
        } else {
            showError('删除失败: ' + (result?.error || '未知错误'));
        }
    } catch (error) {
        showError('删除失败: ' + error.message);
    }
}

function exportWalletsAsJSON() {
    const dataStr = 'data:text/json;charset=utf-8,' + encodeURIComponent(JSON.stringify(walletsCache, null, 2));
    const downloadAnchor = document.createElement('a');
    downloadAnchor.setAttribute('href', dataStr);
    downloadAnchor.setAttribute('download', `wallets-${new Date().toISOString().slice(0,19).replace(/[:T]/g,'-')}.json`);
    document.body.appendChild(downloadAnchor);
    downloadAnchor.click();
    downloadAnchor.remove();
}

async function loadParamSchemesForConfig(selectedSchemeId) {
    try {
        const schemes = await apiRequest(CONFIG.API_ENDPOINTS.PARAM_SCHEMES);
        if (!schemes) return;
        
        const select = document.getElementById('cfgWalletParamScheme');
        if (!select) return;
        
        select.innerHTML = '<option value="">不绑定参数模板</option>';
        schemes.forEach(scheme => {
            const option = document.createElement('option');
            option.value = scheme.id;
            option.textContent = `${scheme.name}${scheme.isDefault ? ' (默认)' : ''}`;
            select.appendChild(option);
        });
        
        if (selectedSchemeId) {
            select.value = selectedSchemeId;
        }
    } catch (error) {
        console.error('加载参数模板失败:', error);
    }
}

// 原有的loadWallets函数保持兼容
function loadWalletsLegacy() {
    fetch('/api/wallets', {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        
        const walletSelect = document.getElementById('walletSelect');
        if (!walletSelect) return;
        
        walletSelect.innerHTML = '<option value="">选择钱包...</option>';
        
        if (data.length === 0) {
            document.getElementById('noWalletAlert')?.classList.remove('d-none');
            return;
        }
        
        document.getElementById('noWalletAlert')?.classList.add('d-none');
        
        data.forEach(wallet => {
            const option = document.createElement('option');
            option.value = wallet.address;
            option.textContent = `${wallet.name || '未命名'} (${wallet.address.substring(0, 8)}...)`;
            walletSelect.appendChild(option);
        });
    })
    .catch(error => {
        console.error('加载钱包列表失败:', error);
        showError('加载钱包列表失败，请刷新页面重试');
    });
}

// 加载钱包详情 - 优化错误处理
async function loadWalletDetails(address) {
    try {
        const data = await apiRequest(`${CONFIG.API_ENDPOINTS.WALLETS}/${address}`);
        if (!data) return;
        
        if (data.error) {
            showError(data.error);
            return;
        }
        
        currentWalletAddress = address;
        
        // 安全地填充钱包详情
        const elements = {
            'walletName': data.name,
            'walletAddress': data.address,
            'walletType': data.type === 'high_frequency' ? '高频交易' : '低频交易'
        };
        
        Object.entries(elements).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element) element.textContent = value;
        });
        
        const inputs = {
            'takeProfitPct': data.take_profit_pct,
            'stopLossPct': data.stop_loss_pct
        };
        
        Object.entries(inputs).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element) element.value = value;
        });
        
        const checkboxes = {
            'walletEnabled': data.enabled,
            'walletFollowSell': data.follow_sell
        };
        
        Object.entries(checkboxes).forEach(([id, checked]) => {
            const element = document.getElementById(id);
            if (element) element.checked = checked;
        });
        
        await loadParamSchemesForWallet(data.param_scheme_id);
        
        document.getElementById('walletInfo')?.classList.remove('d-none');
        document.getElementById('noWalletAlert')?.classList.add('d-none');
    } catch (error) {
        console.error('加载钱包详情失败:', error);
        showError('加载钱包详情失败，请重试');
    }
}

// 为钱包加载参数模板列表 - 重构使用async/await
async function loadParamSchemesForWallet(selectedSchemeId) {
    try {
        const schemes = await apiRequest(CONFIG.API_ENDPOINTS.PARAM_SCHEMES);
        if (!schemes) return;
        
        const schemeSelect = document.getElementById('walletParamScheme');
        if (!schemeSelect) return;
        
        schemeSelect.innerHTML = '<option value="">不绑定参数模板</option>';
        
        schemes.forEach(scheme => {
            const option = document.createElement('option');
            option.value = scheme.id;
            option.textContent = `${scheme.name}${scheme.isDefault ? ' (默认)' : ''}`;
            schemeSelect.appendChild(option);
        });
        
        if (selectedSchemeId) {
            schemeSelect.value = selectedSchemeId;
        }
    } catch (error) {
        console.error('加载参数模板列表失败:', error);
    }
}

// 为卡片加载参数模板名称
async function loadParamSchemeNameForCard(walletAddress, schemeId) {
    try {
        const schemes = await apiRequest(CONFIG.API_ENDPOINTS.PARAM_SCHEMES);
        if (!schemes) return;
        
        const scheme = schemes.find(s => s.id === schemeId);
        const element = document.getElementById(`paramScheme_${walletAddress}`);
        
        if (element) {
            if (scheme) {
                element.innerHTML = `${escapeHtml(scheme.name)}${scheme.is_default ? ' <span class="badge bg-primary" style="font-size: 0.7rem;">默认</span>' : ''}`;
            } else {
                element.textContent = '模板不存在';
                element.style.color = '#dc3545';
            }
        }
    } catch (error) {
        console.error('加载参数模板名称失败:', error);
        const element = document.getElementById(`paramScheme_${walletAddress}`);
        if (element) {
            element.textContent = '加载失败';
            element.style.color = '#dc3545';
        }
    }
}

// 隐藏钱包详情
function hideWalletDetails() {
    document.getElementById('walletInfo').classList.add('d-none');
    currentWalletAddress = '';
}

// 更新钱包设置
function updateWallet() {
    if (!currentWalletAddress) return;
    
    const walletData = {
        address: currentWalletAddress,
        name: document.getElementById('walletName').textContent,
        type: document.getElementById('walletType').textContent === '高频交易' ? 'high_frequency' : 'low_frequency',
        take_profit_pct: parseFloat(document.getElementById('takeProfitPct').value),
        stop_loss_pct: parseFloat(document.getElementById('stopLossPct').value),
        enabled: document.getElementById('walletEnabled').checked,
        follow_sell: document.getElementById('walletFollowSell').checked,
        param_scheme_id: document.getElementById('walletParamScheme').value || null
    };
    
    fetch(`/api/wallets/${currentWalletAddress}`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify(walletData)
    })
    .then(response => {
        console.log('钱包更新响应状态:', response.status);
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json().catch(err => {
            console.error('解析响应JSON失败:', err);
            return { error: '服务器响应格式错误' };
        });
    })
    .then(data => {
        if (!data) return;
        console.log('钱包更新响应数据:', data);
        
        if (data.status === 'success' || data.message) {
            showSuccess(data.message || '钱包设置已更新');
            loadWallets(); // 重新加载钱包列表
        } else {
            showError('更新钱包设置失败: ' + (data.error || '未知错误'));
        }
    })
    .catch(error => {
        console.error('更新钱包设置失败:', error);
        showError('更新钱包设置失败: ' + error.message);
    });
}

// 删除钱包
function deleteWallet() {
    if (!currentWalletAddress) return;
    
    if (!confirm(`确定要删除钱包 ${document.getElementById('walletName').textContent} 吗？此操作不可撤销。`)) {
        return;
    }
    
    fetch(`/api/wallets/${currentWalletAddress}`, {
        method: 'DELETE',
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        console.log('删除钱包响应数据:', data);
        
        if (data.status === 'success' || data.message === '钱包删除成功' || data.message) {
            showSuccess(data.message || '钱包已删除');
            hideWalletDetails();
            loadWallets(); // 重新加载钱包列表
        } else {
            showError('删除钱包失败: ' + (data.error || '未知错误'));
        }
    })
    .catch(error => {
        console.error('删除钱包失败:', error);
        showError('删除钱包失败，请重试');
    });
}

// 添加新钱包
function addNewWallet() {
    const walletData = {
        address: document.getElementById('newWalletAddress').value.trim(),
        name: document.getElementById('newWalletName').value.trim(),
        type: document.getElementById('newWalletType').value,
        take_profit_pct: parseFloat(document.getElementById('newTakeProfitPct').value),
        stop_loss_pct: parseFloat(document.getElementById('newStopLossPct').value),
        enabled: document.getElementById('newWalletEnabled').checked,
        follow_sell: document.getElementById('newWalletFollowSell').checked
    };
    
    // 验证输入
    if (!walletData.address || !walletData.name) {
        showError('请填写所有必填字段');
        return;
    }
    
    // 验证 Solana 地址格式
    if (!isValidSolanaAddress(walletData.address)) {
        showError('无效的 Solana 钱包地址格式');
        return;
    }
    
    fetch('/api/wallets', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify(walletData)
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        console.log('添加钱包响应数据:', data);
        
        if (data.status === 'success' || data.message === '钱包创建成功' || data.message) {
            // 关闭模态框
            const modal = bootstrap.Modal.getInstance(document.getElementById('addWalletModal'));
            modal.hide();
            
            // 清空表单
            document.getElementById('addWalletForm').reset();
            
            showSuccess(data.message || '钱包已添加');
            loadWallets(); // 重新加载钱包列表
        } else {
            showError('添加钱包失败: ' + (data.error || '未知错误'));
        }
    })
    .catch(error => {
        console.error('添加钱包失败:', error);
        showError('添加钱包失败，请重试');
    });
}

// 加载交易设置
function loadTradeSettings() {
    fetch('/api/trade-settings', {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(settings => {
        if (!settings) return;
        
        // 只填充5个交易开关设置
        document.getElementById('pumpTradeEnable').checked = settings.pump_trade_enable;
        document.getElementById('pumpAmmTradeEnable').checked = settings.pump_amm_trade_enable;
        document.getElementById('raydiumTradeEnable').checked = settings.raydium_trade_enable;
        document.getElementById('launchpadTradeEnable').checked = settings.launchpad_trade_enable;
        document.getElementById('repeatTradeEnable').checked = settings.repeat_trade_enable;
    })
    .catch(error => {
        console.error('加载交易设置失败:', error);
        showError('加载交易设置失败，请刷新页面重试');
    });
}

// 加载参数模板列表
function loadParamSchemes() {
    fetch('/api/param-schemes', {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(schemes => {
        if (!schemes) return;
        
        const schemeSelect = document.getElementById('schemeSelect');
        schemeSelect.innerHTML = '<option value="">选择参数模板...</option>';
        
        if (schemes.length === 0) {
            // 显示无参数模板提示
            document.getElementById('noSchemeAlert').classList.remove('d-none');
            return;
        }
        
        // 隐藏无参数模板提示
        document.getElementById('noSchemeAlert').classList.add('d-none');
        
        // 添加参数模板选项
        schemes.forEach(scheme => {
            const option = document.createElement('option');
            option.value = scheme.id;
            option.textContent = `${scheme.name}${scheme.isDefault ? ' (默认)' : ''}`;
            schemeSelect.appendChild(option);
        });
    })
    .catch(error => {
        console.error('加载参数模板列表失败:', error);
        showError('加载参数模板列表失败，请刷新页面重试');
    });
}

// 加载参数模板详情
function loadSchemeDetails(schemeId) {
    fetch(`/api/param-schemes/${schemeId}`, {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(scheme => {
        if (!scheme) return;
        console.info('加载参数模板详情:', scheme);
        // 填充参数模板信息
        document.getElementById('schemeName').textContent = scheme.name;
        document.getElementById('schemeDescription').textContent = scheme.description || '无描述';
        document.getElementById('schemeId').textContent = scheme.id;
        
        // 填充参数模板设置
        document.getElementById('schemeSellDelayTime').value = scheme.sell_delay_time;
        document.getElementById('schemeMinFollowTriggerAmount').value = scheme.min_follow_trigger_amount;
        document.getElementById('schemeMaxFollowTriggerAmount').value = scheme.max_follow_trigger_amount;
        document.getElementById('schemeMinPoolSize').value = scheme.min_pool_size;
        document.getElementById('schemeMaxPoolSize').value = scheme.max_pool_size;
        document.getElementById('schemeTokenAgeMinMinutes').value = scheme.token_age_min_minutes || 0;
        document.getElementById('schemeTokenAgeMaxMinutes').value = scheme.token_age_max_minutes || 0;
        //document.getElementById('schemeMinBuyAmount').value = scheme.min_buy_amount;
        //document.getElementById('schemeMaxBuyAmount').value = scheme.max_buy_amount;
        document.getElementById('schemeBuySolAmount').value = scheme.buy_sol_amount;
        //document.getElementById('schemeSellPercentage').value = scheme.sell_percentage;
        //document.getElementById('schemeStopLossPercentage').value = scheme.stop_loss_percentage;
        //document.getElementById('schemeTakeProfitPercentage').value = scheme.take_profit_percentage;
        document.getElementById('schemeBuySlippage').value = scheme.buy_slippage;
        document.getElementById('schemeSellSlippage').value = scheme.sell_slippage;
        document.getElementById('schemeBuyPriorityFee').value = scheme.buy_priority_fee;
        document.getElementById('schemeBuyTipFee').value = scheme.buy_tip_fee;
        document.getElementById('schemeSellPriorityFee').value = scheme.sell_priority_fee;
        document.getElementById('schemeSellTipFee').value = scheme.sell_tip_fee;
        document.getElementById('schemeIsDefault').checked = scheme.is_default;
        
        // 显示参数模板详情
        document.getElementById('schemeInfo').classList.remove('d-none');
    })
    .catch(error => {
        console.error('加载参数模板详情失败:', error);
        showError('加载参数模板详情失败，请刷新页面重试');
    });
}

// 隐藏参数模板详情
function hideSchemeDetails() {
    document.getElementById('schemeInfo').classList.add('d-none');
}

// 创建新参数模板
function addNewScheme() {
    // 收集表单数据
    const newScheme = {
        name: document.getElementById('newSchemeName').value,
        description: document.getElementById('newSchemeDescription').value,
        is_default: document.getElementById('newSchemeIsDefault').checked,
        sell_delay_time: parseInt(document.getElementById('newSchemeSellDelayTime').value),
        min_follow_trigger_amount: parseFloat(document.getElementById('newSchemeMinFollowTriggerAmount').value),
        max_follow_trigger_amount: parseFloat(document.getElementById('newSchemeMaxFollowTriggerAmount').value),
        min_pool_size: parseFloat(document.getElementById('newSchemeMinPoolSize').value),
        max_pool_size: parseFloat(document.getElementById('newSchemeMaxPoolSize').value),
        //min_buy_amount: parseFloat(document.getElementById('newSchemeMinBuyAmount').value),
        //max_buy_amount: parseFloat(document.getElementById('newSchemeMaxBuyAmount').value),
        buy_sol_amount: parseFloat(document.getElementById('newSchemeBuySolAmount').value),
        //sell_percentage: parseFloat(document.getElementById('newSchemeSellPercentage').value),
        //stop_loss_percentage: parseFloat(document.getElementById('newSchemeStopLossPercentage').value),
        //take_profit_percentage: parseFloat(document.getElementById('newSchemeTakeProfitPercentage').value),
        buy_slippage: parseFloat(document.getElementById('newSchemeBuySlippage').value),
        sell_slippage: parseFloat(document.getElementById('newSchemeSellSlippage').value),
        buy_priority_fee: parseFloat(document.getElementById('newSchemeBuyPriorityFee').value),
        buy_tip_fee: parseFloat(document.getElementById('newSchemeBuyTipFee').value),
        sell_priority_fee: parseFloat(document.getElementById('newSchemeSellPriorityFee').value),
        sell_tip_fee: parseFloat(document.getElementById('newSchemeSellTipFee').value)
    };
    
    // 验证必填字段
    if (!newScheme.name) {
        showError('模板名称不能为空');
        return;
    }
    
    // 发送请求创建新参数模板
    fetch('/api/param-schemes', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify(newScheme)
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        
        // 关闭模态框
        const modal = bootstrap.Modal.getInstance(document.getElementById('addSchemeModal'));
        modal.hide();
        
        // 重新加载参数模板列表
        loadParamSchemes();
        
        // 显示成功提示
        showSuccess('参数模板创建成功');
    })
    .catch(error => {
        console.error('创建参数模板失败:', error);
        showError('创建参数模板失败，请重试');
    });
}

// 更新参数模板
function updateScheme() {
    // 获取当前选中的参数模板 ID
    const schemeId = document.getElementById('schemeId').textContent;
    if (!schemeId) {
        showError('未选择参数模板');
        return;
    }
    
    // 收集表单数据
    const updatedScheme = {
        id: schemeId,
        name: document.getElementById('schemeName').textContent,
        description: document.getElementById('schemeDescription').textContent === '无描述' ? '' : document.getElementById('schemeDescription').textContent,
        is_default: document.getElementById('schemeIsDefault').checked,
        sell_delay_time: parseInt(document.getElementById('schemeSellDelayTime').value),
        min_follow_trigger_amount: parseFloat(document.getElementById('schemeMinFollowTriggerAmount').value),
        max_follow_trigger_amount: parseFloat(document.getElementById('schemeMaxFollowTriggerAmount').value),
        min_pool_size: parseFloat(document.getElementById('schemeMinPoolSize').value),
        max_pool_size: parseFloat(document.getElementById('schemeMaxPoolSize').value),
        token_age_min_minutes: parseInt(document.getElementById('schemeTokenAgeMinMinutes').value) || 0,
        token_age_max_minutes: parseInt(document.getElementById('schemeTokenAgeMaxMinutes').value) || 0,
        //min_buy_amount: parseFloat(document.getElementById('schemeMinBuyAmount').value),
        //max_buy_amount: parseFloat(document.getElementById('schemeMaxBuyAmount').value),
        buy_sol_amount: parseFloat(document.getElementById('schemeBuySolAmount').value),
        //sell_percentage: parseFloat(document.getElementById('schemeSellPercentage').value),
        //stop_loss_percentage: parseFloat(document.getElementById('schemeStopLossPercentage').value),
        //take_profit_percentage: parseFloat(document.getElementById('schemeTakeProfitPercentage').value),
        buy_slippage: parseFloat(document.getElementById('schemeBuySlippage').value),
        sell_slippage: parseFloat(document.getElementById('schemeSellSlippage').value),
        buy_priority_fee: parseFloat(document.getElementById('schemeBuyPriorityFee').value),
        buy_tip_fee: parseFloat(document.getElementById('schemeBuyTipFee').value),
        sell_priority_fee: parseFloat(document.getElementById('schemeSellPriorityFee').value),
        sell_tip_fee: parseFloat(document.getElementById('schemeSellTipFee').value)
    };
    
    // 发送请求更新参数模板
    fetch(`/api/param-schemes/${schemeId}`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify(updatedScheme)
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        
        // 重新加载参数模板列表
        loadParamSchemes();
        
        // 显示成功提示
        showSuccess('参数模板更新成功');
    })
    .catch(error => {
        console.error('更新参数模板失败:', error);
        showError('更新参数模板失败，请重试');
    });
}

// 删除参数模板
function deleteScheme() {
    // 获取当前选中的参数模板 ID
    const schemeId = document.getElementById('schemeId').textContent;
    if (!schemeId) {
        showError('未选择参数模板');
        return;
    }
    
    // 确认删除
    if (!confirm('确定要删除此参数模板吗？删除后无法恢复。')) {
        return;
    }
    
    // 发送请求删除参数模板
    fetch(`/api/param-schemes/${schemeId}`, {
        method: 'DELETE',
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        // 重新加载参数模板列表
        loadParamSchemes();
        
        // 隐藏参数模板详情
        hideSchemeDetails();
        
        // 显示成功提示
        showSuccess('参数模板删除成功');
    })
    .catch(error => {
        console.error('删除参数模板失败:', error);
        showError('删除参数模板失败，请重试');
    });
}

// 保存交易设置
function saveTradeSettings() {
    console.log('开始保存交易设置...');
    
    // 只保留5个交易开关设置
    const settings = {
        pump_trade_enable: document.getElementById('pumpTradeEnable').checked,
        pump_amm_trade_enable: document.getElementById('pumpAmmTradeEnable').checked,
        raydium_trade_enable: document.getElementById('raydiumTradeEnable').checked,
        launchpad_trade_enable: document.getElementById('launchpadTradeEnable').checked,
        repeat_trade_enable: document.getElementById('repeatTradeEnable').checked
    };
    
    console.log('发送的设置数据:', settings);
    
    fetch('/api/trade-settings', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify(settings)
    })
    .then(response => {
        console.log('收到响应:', response.status, response.statusText);
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        console.log('解析的响应数据:', data);
        if (!data) {
            console.log('没有数据返回');
            return;
        }
        
        if (data.status === 'success') {
            console.log('准备显示成功提示');
            showSuccess('交易设置已保存');
        } else {
            console.log('准备显示错误提示');
            showError('保存交易设置失败: ' + (data.error || '未知错误'));
        }
    })
    .catch(error => {
        console.error('保存交易设置失败:', error);
        showError('保存交易设置失败，请重试');
    });
}

// 退出登录
function logout() {
    // 清除本地存储的认证信息
    localStorage.removeItem('auth_token');
    localStorage.removeItem('username');
    
    // 重定向到登录页面
    window.location.href = '/login';
}

// 显示成功消息
function showSuccess(message) {
    console.log('显示成功消息:', message);
    
    // 使用更简单的方式显示通知
    const alertDiv = document.createElement('div');
    alertDiv.className = 'alert alert-success alert-dismissible fade show position-fixed top-0 end-0 m-3';
    alertDiv.setAttribute('role', 'alert');
    alertDiv.style.zIndex = '9999';
    alertDiv.innerHTML = `
        <i class="bi bi-check-circle me-2"></i> ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    `;
    
    document.body.appendChild(alertDiv);
    
    // 3秒后自动移除
    setTimeout(() => {
        alertDiv.remove();
    }, 5000);
    
    try {
        // 尝试使用 Bootstrap Toast
        const toast = document.getElementById('successAlert');
        if (toast) {
            const successMessage = document.getElementById('successMessage');
            if (successMessage) {
                successMessage.innerHTML = `<i class="bi bi-check-circle me-2"></i> ${message}`;
            }
            
            const bsToast = new bootstrap.Toast(toast);
            bsToast.show();
        }
    } catch (error) {
        console.error('显示 Toast 失败:', error);
    }
}

// 显示错误提示
function showError(message) {
    console.log('显示错误消息:', message);
    
    // 使用更简单的方式显示通知
    const alertDiv = document.createElement('div');
    alertDiv.className = 'alert alert-danger alert-dismissible fade show position-fixed top-0 end-0 m-3';
    alertDiv.setAttribute('role', 'alert');
    alertDiv.style.zIndex = '9999';
    alertDiv.innerHTML = `
        <i class="bi bi-exclamation-triangle me-2"></i> ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    `;
    
    document.body.appendChild(alertDiv);
    
    // 3秒后自动移除
    setTimeout(() => {
        alertDiv.remove();
    }, 3000);
    
    try {
        // 尝试使用 Bootstrap Toast
        const toast = document.getElementById('errorAlert');
        if (toast) {
            const errorMessage = document.getElementById('errorMessage');
            if (errorMessage) {
                errorMessage.textContent = message;
            }
            
            const bsToast = new bootstrap.Toast(toast);
            bsToast.show();
        }
    } catch (error) {
        console.error('显示 Toast 失败:', error);
    }
}

// 验证 Solana 地址格式
function isValidSolanaAddress(address) {
    // Solana 地址是 32-44 个字符的 Base58 编码字符串
    const base58Regex = /^[1-9A-HJ-NP-Za-km-z]{32,44}$/;
    return base58Regex.test(address);
}

// 强制滑点范围在0-1之间
function enforceSlippageRange(input, showMessage = false) {
    let value = parseFloat(input.value);
    
    // 如果不是数字，设置为0
    if (isNaN(value)) {
        input.value = "0";
        return;
    }
    
    // 如果小于0，设置为0
    if (value < 0) {
        input.value = "0";
        if (showMessage) {
            showError("滑点不能小于0");
        }
        return;
    }
    
    // 如果大于1，设置为1
    if (value > 1) {
        input.value = "1";
        if (showMessage) {
            showError("滑点不能大于1");
        }
        return;
    }
    
    // 保留两位小数
    if (input.value.indexOf('.') !== -1 && input.value.split('.')[1].length > 2) {
        input.value = value.toFixed(2);
    }
}

// 验证私钥格式
function isValidPrivateKey(privateKey) {
    // 64位十六进制私钥
    const hexRegex = /^[0-9a-fA-F]{64}$/;
     
    return hexRegex.test(privateKey.trim());
}

// 加载私钥钱包列表
function loadPrivateKeyWallets() {
    fetch('/api/wallets/private-keys', {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        
        const walletList = document.getElementById('privateKeyWalletList');
        
        if (data.length === 0) {

            return;
        }
        
        // 隐藏无钱包提示
        document.getElementById('noPrivateKeyWalletAlert').classList.add('d-none');
        
        // 清空现有内容
        walletList.innerHTML = '';
        
        // 创建钱包列表
        const table = document.createElement('table');
        table.className = 'table table-hover';
        table.innerHTML = `
            <thead>
                <tr>
                    <th>钱包名称</th>
                    <th>钱包地址</th>
                    
         
                </tr>
            </thead>
            <tbody id="privateKeyWalletTableBody">
            </tbody>
        `;
        
        walletList.appendChild(table);
        
        const tbody = document.getElementById('privateKeyWalletTableBody');
        
        data.forEach(wallet => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${wallet.name}</td>
                <td><span class="text-monospace">${wallet.address}</span></td>
    
            `;
            tbody.appendChild(row);
            
            // 添加删除按钮事件
            const deleteBtn = row.querySelector('.delete-wallet-btn');
            if (deleteBtn) {
                deleteBtn.addEventListener('click', function() {
                    const address = this.getAttribute('data-address');
                    if (address) {
                        deletePrivateKeyWallet(address);
                    }
                });
            }
        });
    })
    .catch(error => {
        console.error('加载私钥钱包列表失败:', error);
        showError('加载私钥钱包列表失败，请刷新页面重试');
    });
}

// 导入交易钱包
function importPrivateKeyWallet() {
    const privateKey = document.getElementById('privateKeyInput').value.trim();
    const walletName = document.getElementById('pkWalletName').value.trim();
    
    // 验证输入
    if (!privateKey) {
        showError('请输入私钥或助记词');
        return;
    }
    
    
    if (!walletName) {
        showError('请输入钱包名称');
        return;
    }
    
    // 显示加载状态
    const importBtn = document.getElementById('confirmImportPrivateKeyBtn');
    const originalText = importBtn.textContent;
    importBtn.disabled = true;
    importBtn.innerHTML = '<span class="spinner-border spinner-border-sm me-2" role="status"></span>导入中...';
    
    // 构建请求数据，只包含必要字段
    const walletData = {
        private_key: privateKey,
        name: walletName
    };
    
    // 发送导入请求
    fetch('/api/wallets/import-private-key', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify(walletData)
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        // 恢复按钮状态
        importBtn.disabled = false;
        importBtn.textContent = originalText;
        
        if (!data) return;
        
        if (data.status === 'success' || data.message) {
            // 关闭模态框
            const modal = bootstrap.Modal.getInstance(document.getElementById('importPrivateKeyModal'));
            modal.hide();
            
            // 清空表单
            document.getElementById('importPrivateKeyForm').reset();
            
            showSuccess(data.message || '钱包导入成功');
            loadPrivateKeyWallets(); // 重新加载私钥钱包列表
        } else {
            showError('导入钱包失败: ' + (data.error || '未知错误'));
        }
    })
    .catch(error => {
        // 恢复按钮状态
        importBtn.disabled = false;
        importBtn.textContent = originalText;
        
        console.error('导入钱包失败:', error);
        showError('导入钱包失败，请重试');
    });
}

// 删除私钥钱包
function deletePrivateKeyWallet(address) {
    if (!confirm(`确定要删除私钥钱包 ${address.substring(0, 8)}... 吗？此操作不可撤销。`)) {
        return;
    }
    
    fetch(`/api/wallets/private-key/${address}`, {
        method: 'DELETE',
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        
        if (data.status === 'success' || data.message) {
            showSuccess(data.message || '私钥钱包已删除');
            loadPrivateKeyWallets(); // 重新加载私钥钱包列表
        } else {
            showError('删除私钥钱包失败: ' + (data.error || '未知错误'));
        }
    })
    .catch(error => {
        console.error('删除私钥钱包失败:', error);
        showError('删除私钥钱包失败，请重试');
    });
}

    // 强制滑点范围在0-1之间
    function enforceSlippageRange(input, showMessage = false) {
        let value = parseFloat(input.value);
        
        // 如果不是数字，设置为0
        if (isNaN(value)) {
            input.value = "0";
            return;
        }
        
        // 如果小于0，设置为0
        if (value < 0) {
            input.value = "0";
            if (showMessage) {
                showError("滑点不能小于0");
            }
            return;
        }
        
        // 如果大于1，设置为1
        if (value > 1) {
            input.value = "1";
            if (showMessage) {
                showError("滑点不能大于1");
            }
            return;
        }
        
        // 保留两位小数
        if (input.value.indexOf('.') !== -1 && input.value.split('.')[1].length > 2) {
            
            // 清空表单
            document.getElementById('importPrivateKeyForm').reset();
            
            showSuccess(data.message || '钱包导入成功');
            loadPrivateKeyWallets(); // 重新加载私钥钱包列表
        } else {
            showError('导入钱包失败: ' + (data.error || '未知错误'));
        }
    }

// 删除私钥钱包
function deletePrivateKeyWallet(address) {
    if (!confirm(`确定要删除私钥钱包 ${address.substring(0, 8)}... 吗？此操作不可撤销。`)) {
        return;
    }
    
    fetch(`/api/wallets/private-key/${address}`, {
        method: 'DELETE',
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        
        if (data.status === 'success' || data.message) {
            showSuccess(data.message || '私钥钱包已删除');
            loadPrivateKeyWallets(); // 重新加载私钥钱包列表
        } else {
            showError('删除私钥钱包失败: ' + (data.error || '未知错误'));
        }
    })
    .catch(error => {
        console.error('删除私钥钱包失败:', error);
        showError('删除私钥钱包失败，请重试');
    });
}

// 显示钱包私钥管理界面（可选功能）
function showPrivateKeyManagement() {
    // 创建私钥管理模态框（如果需要单独的私钥管理界面）
    const modalHTML = `
    <div class="modal fade" id="privateKeyManagementModal" tabindex="-1">
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">钱包私钥管理</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <div class="alert alert-info">
                        <i class="bi bi-info-circle me-2"></i>
                        您可以在此处管理已导入的钱包私钥。出于安全考虑，私钥只显示部分信息。
                    </div>
                    <div id="walletKeysList">
                        <!-- 动态加载钱包列表 -->
                    </div>
                </div>
            </div>
        </div>
    </div>`;
    
    // 添加到页面（如果不存在）
    if (!document.getElementById('privateKeyManagementModal')) {
        document.body.insertAdjacentHTML('beforeend', modalHTML);
    }
    
    // 显示模态框
}

// 加载全局卖出设置
function loadGlobalSellSettings() {
    fetch('/api/redis-config', {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => response.json())
    .then(data => {
        // 填充全局卖出设置表单
        document.getElementById('capitalRecoveryPercent').value = data.capital_recovery_percent || 0;
        document.getElementById('devSellEnable').checked = data.dev_sell || false;
    })
    .catch(error => {
        console.error('加载全局卖出设置失败:', error);
        showToast('加载全局卖出设置失败', 'error');
    });
}

// 保存全局卖出设置
function saveGlobalSellSettings() {
    const capitalRecoveryPercent = parseFloat(document.getElementById('capitalRecoveryPercent').value) || 0;
    const devSell = document.getElementById('devSellEnable').checked;
    
    // 验证输入
    if (capitalRecoveryPercent < 0 || capitalRecoveryPercent > 300) {
        showToast('抽回本金盈利百分比必须在0-300之间', 'error');
        return;
    }
    
    const updates = {
        capital_recovery_percent: capitalRecoveryPercent,
        dev_sell: devSell
    };
    
    fetch('/api/redis-config', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify(updates)
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'success') {
            showSuccess('全局卖出设置保存成功');
        } else {
            showError('保存失败: ' + (data.message || '未知错误'));
        }
    })
    .catch(error => {
        console.error('保存全局卖出设置失败:', error);
        showError('保存全局卖出设置失败');
    });
}

// 显示提示消息
function showToast(message, type) {
    const successAlert = document.getElementById('successAlert');
    const errorAlert = document.getElementById('errorAlert');
    
    // 隐藏所有提示框
    successAlert.classList.add('d-none');
    errorAlert.classList.add('d-none');
    
    if (type === 'success') {
        successAlert.textContent = message;
        successAlert.classList.remove('d-none');
        // 3秒后自动隐藏
        setTimeout(() => {
            successAlert.classList.add('d-none');
        }, 3000);
    } else if (type === 'error') {
        errorAlert.textContent = message;
        errorAlert.classList.remove('d-none');
        // 5秒后自动隐藏
        setTimeout(() => {
            errorAlert.classList.add('d-none');
        }, 5000);
    }
}

// ==================== 黑名单管理功能 ====================

// 加载黑名单列表
function loadBlacklist() {
    fetch(CONFIG.API_ENDPOINTS.BLACKLIST, {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => response.json())
    .then(data => {
        displayBlacklist(data);
    })
    .catch(error => {
        console.error('加载黑名单失败:', error);
        showError('加载黑名单失败');
    });
}

// 显示黑名单列表
function displayBlacklist(blacklist) {
    const tbody = document.getElementById('blacklistTableBody');
    const noDataAlert = document.getElementById('noBlacklistAlert');
    const countBadge = document.getElementById('blacklistCountBadge');
    
    // 更新计数
    countBadge.textContent = `${blacklist.length} 个`;
    
    if (blacklist.length === 0) {
        tbody.innerHTML = '';
        noDataAlert.classList.remove('d-none');
        return;
    }
    
    noDataAlert.classList.add('d-none');
    
    tbody.innerHTML = blacklist.map((item, index) => {
        const addedDate = new Date(item.added_at * 1000).toLocaleDateString('zh-CN');
        const statusBadge = item.is_enabled 
            ? '<span class="badge bg-success">启用</span>' 
            : '<span class="badge bg-secondary">禁用</span>';
        
        return `
            <tr>
                <td>${index + 1}</td>
                <td>
                    <code class="text-break">${item.address}</code>
                </td>
                <td>${item.note || '-'}</td>
                <td>${addedDate}</td>
                <td>${statusBadge}</td>
                <td>
                    <div class="btn-group btn-group-sm" role="group">
                        <button class="btn btn-outline-warning" onclick="editBlacklist('${item.address}')" title="编辑">
                            <i class="bi bi-pencil"></i>
                        </button>
                        <button class="btn btn-outline-danger" onclick="deleteBlacklist('${item.address}')" title="删除">
                            <i class="bi bi-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    }).join('');
}

// 添加黑名单地址
function addBlacklist() {
    const address = document.getElementById('blacklistAddress').value.trim();
    const note = document.getElementById('blacklistNote').value.trim();
    
    if (!address) {
        showError('请输入钱包地址');
        return;
    }
    
    fetch(CONFIG.API_ENDPOINTS.BLACKLIST, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify({ address, note })
    })
    .then(response => response.json())
    .then(data => {
        if (data.message) {
            showSuccess('黑名单地址添加成功');
            // 关闭模态框
            const modal = bootstrap.Modal.getInstance(document.getElementById('addBlacklistModal'));
            modal.hide();
            // 清空表单
            document.getElementById('addBlacklistForm').reset();
            // 重新加载列表
            loadBlacklist();
        } else {
            showError(data.error || '添加失败');
        }
    })
    .catch(error => {
        console.error('添加黑名单失败:', error);
        showError('添加黑名单失败');
    });
}

// 批量添加黑名单地址
function batchAddBlacklist() {
    const addresses = document.getElementById('batchBlacklistAddresses').value.trim();
    const note = document.getElementById('batchBlacklistNote').value.trim();
    
    if (!addresses) {
        showError('请输入钱包地址列表');
        return;
    }
    
    fetch(`${CONFIG.API_ENDPOINTS.BLACKLIST}/batch`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify({ addresses, note })
    })
    .then(response => response.json())
    .then(data => {
        if (data.message) {
            const successCount = data.success_count || 0;
            const failedCount = data.failed_count || 0;
            showSuccess(`批量添加完成：成功 ${successCount} 个，失败 ${failedCount} 个`);
            // 关闭模态框
            const modal = bootstrap.Modal.getInstance(document.getElementById('batchAddBlacklistModal'));
            modal.hide();
            // 清空表单
            document.getElementById('batchAddBlacklistForm').reset();
            // 重新加载列表
            loadBlacklist();
        } else {
            showError(data.error || '批量添加失败');
        }
    })
    .catch(error => {
        console.error('批量添加黑名单失败:', error);
        showError('批量添加黑名单失败');
    });
}

// 编辑黑名单地址
function editBlacklist(address) {
    // 获取黑名单详情
    fetch(`${CONFIG.API_ENDPOINTS.BLACKLIST}/${address}`, {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => response.json())
    .then(data => {
        // 填充表单
        document.getElementById('editBlacklistAddress').value = data.address;
        document.getElementById('editBlacklistNote').value = data.note || '';
        document.getElementById('editBlacklistEnabled').checked = data.is_enabled;
        
        // 显示模态框
        const modal = new bootstrap.Modal(document.getElementById('editBlacklistModal'));
        modal.show();
    })
    .catch(error => {
        console.error('获取黑名单详情失败:', error);
        showError('获取黑名单详情失败');
    });
}

// 保存黑名单编辑
function saveBlacklistEdit() {
    const address = document.getElementById('editBlacklistAddress').value;
    const note = document.getElementById('editBlacklistNote').value.trim();
    const isEnabled = document.getElementById('editBlacklistEnabled').checked;
    
    fetch(`${CONFIG.API_ENDPOINTS.BLACKLIST}/${address}`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify({ note, is_enabled: isEnabled })
    })
    .then(response => response.json())
    .then(data => {
        if (data.message) {
            showSuccess('黑名单更新成功');
            // 关闭模态框
            const modal = bootstrap.Modal.getInstance(document.getElementById('editBlacklistModal'));
            modal.hide();
            // 重新加载列表
            loadBlacklist();
        } else {
            showError(data.error || '更新失败');
        }
    })
    .catch(error => {
        console.error('更新黑名单失败:', error);
        showError('更新黑名单失败');
    });
}

// 删除黑名单地址
function deleteBlacklist(address) {
    if (!confirm(`确定要删除黑名单地址吗？\n${address}`)) {
        return;
    }
    
    fetch(`${CONFIG.API_ENDPOINTS.BLACKLIST}/${address}`, {
        method: 'DELETE',
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => response.json())
    .then(data => {
        if (data.message) {
            showSuccess('黑名单地址删除成功');
            // 重新加载列表
            loadBlacklist();
        } else {
            showError(data.error || '删除失败');
        }
    })
    .catch(error => {
        console.error('删除黑名单失败:', error);
        showError('删除黑名单失败');
    });
}

// 注册黑名单相关事件处理器
document.addEventListener('DOMContentLoaded', function() {
    // 添加黑名单按钮
    const confirmAddBtn = document.getElementById('confirmAddBlacklistBtn');
    if (confirmAddBtn) {
        confirmAddBtn.addEventListener('click', addBlacklist);
    }
    
    // 批量添加黑名单按钮
    const confirmBatchAddBtn = document.getElementById('confirmBatchAddBlacklistBtn');
    if (confirmBatchAddBtn) {
        confirmBatchAddBtn.addEventListener('click', batchAddBlacklist);
    }
    
    // 保存黑名单编辑按钮
    const confirmEditBtn = document.getElementById('confirmEditBlacklistBtn');
    if (confirmEditBtn) {
        confirmEditBtn.addEventListener('click', saveBlacklistEdit);
    }
});
