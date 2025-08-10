// 全局变量
let currentWalletAddress = '';
let authToken = '';

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
}

// 注册事件处理器
function registerEventHandlers() {
    // 导航链接点击事件
    document.getElementById('walletManagementLink').addEventListener('click', function(e) {
        e.preventDefault();
        showSection('walletManagement');
    });
    
    document.getElementById('paramSchemeLink').addEventListener('click', function(e) {
        e.preventDefault();
        showSection('paramScheme');
    });
    
    document.getElementById('tradeSettingsLink').addEventListener('click', function(e) {
        e.preventDefault();
        showSection('tradeSettings');
    });
    
    // 退出登录按钮
    document.getElementById('logoutBtn').addEventListener('click', function() {
        logout();
    });
    
    // 钱包选择变更事件
    document.getElementById('walletSelect').addEventListener('change', function() {
        const selectedAddress = this.value;
        if (selectedAddress) {
            loadWalletDetails(selectedAddress);
        } else {
            hideWalletDetails();
        }
    });
    
    // 保存钱包设置按钮
    document.getElementById('updateWalletBtn').addEventListener('click', function() {
        updateWallet();
    });
    
    // 删除钱包按钮
    document.getElementById('deleteWalletBtn').addEventListener('click', function() {
        deleteWallet();
    });
    
    // 添加新钱包按钮
    document.getElementById('saveNewWalletBtn').addEventListener('click', function() {
        addNewWallet();
    });
    
    // 保存交易设置按钮
    document.getElementById('saveTradeSettingsBtn').addEventListener('click', function() {
        saveTradeSettings();
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

// 显示指定部分
function showSection(section) {
    // 隐藏所有部分
    document.getElementById('walletManagementSection').classList.add('d-none');
    document.getElementById('paramSchemeSection').classList.add('d-none');
    document.getElementById('tradeSettingsSection').classList.add('d-none');
    
    // 移除所有导航链接的激活状态
    document.querySelectorAll('.nav-link').forEach(link => {
        link.classList.remove('active');
    });
    
    // 显示指定部分
    switch (section) {
        case 'walletManagement':
            document.getElementById('walletManagementSection').classList.remove('d-none');
            document.getElementById('walletManagementLink').classList.add('active');
            break;
        case 'paramScheme':
            document.getElementById('paramSchemeSection').classList.remove('d-none');
            document.getElementById('paramSchemeLink').classList.add('active');
            break;
        case 'tradeSettings':
            document.getElementById('tradeSettingsSection').classList.remove('d-none');
            document.getElementById('tradeSettingsLink').classList.add('active');
            break;
    }
}

// 加载钱包列表
function loadWallets() {
    fetch('/api/wallets', {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
    .then(response => {
        if (response.status === 401) {
            // 未授权，重定向到登录页面
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        
        const walletSelect = document.getElementById('walletSelect');
        walletSelect.innerHTML = '<option value="">选择钱包...</option>';
        
        if (data.length === 0) {
            // 显示无钱包提示
            document.getElementById('noWalletAlert').classList.remove('d-none');
            return;
        }
        
        // 隐藏无钱包提示
        document.getElementById('noWalletAlert').classList.add('d-none');
        
        // 添加钱包选项
        data.forEach(wallet => {
            const option = document.createElement('option');
            option.value = wallet.address;
            option.textContent = `${wallet.name} (${wallet.address.substring(0, 8)}...)`;
            walletSelect.appendChild(option);
        });
    })
    .catch(error => {
        console.error('加载钱包列表失败:', error);
        showError('加载钱包列表失败，请刷新页面重试');
    });
}

// 加载钱包详情
function loadWalletDetails(address) {
    // 先加载钱包详情
    fetch(`/api/wallets/${address}`, {
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
    .then(wallet => {
        if (!wallet) return;
        
        // 保存当前钱包地址
        currentWalletAddress = wallet.address;
        
        // 填充钱包信息
        document.getElementById('walletName').textContent = wallet.name;
        document.getElementById('walletAddress').textContent = wallet.address;
        document.getElementById('walletType').textContent = wallet.type === 'high_frequency' ? '高频交易' : '低频交易';
        document.getElementById('takeProfitPct').value = wallet.take_profit_pct;
        document.getElementById('stopLossPct').value = wallet.stop_loss_pct;
        document.getElementById('exitTimeSec').value = wallet.exit_time_sec;
        document.getElementById('walletEnabled').checked = wallet.enabled;
        
        // 加载参数模板列表并设置选中项
        loadParamSchemesForWallet(wallet.param_scheme_id);
        
        // 显示钱包详情
        document.getElementById('walletInfo').classList.remove('d-none');
    })
    .catch(error => {
        console.error('加载钱包详情失败:', error);
        showError('加载钱包详情失败，请重试');
    });
}

// 为钱包加载参数模板列表
function loadParamSchemesForWallet(selectedSchemeId) {
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
        
        const schemeSelect = document.getElementById('walletParamScheme');
        schemeSelect.innerHTML = '<option value="">不绑定参数模板</option>';
        
        if (schemes.length === 0) {
            return;
        }
        
        // 添加参数模板选项
        schemes.forEach(scheme => {
            const option = document.createElement('option');
            option.value = scheme.id;
            option.textContent = `${scheme.name}${scheme.isDefault ? ' (默认)' : ''}`;
            schemeSelect.appendChild(option);
        });
        
        // 如果有选中的参数模板，设置选中状态
        if (selectedSchemeId) {
            schemeSelect.value = selectedSchemeId;
        }
    })
    .catch(error => {
        console.error('加载参数模板列表失败:', error);
    });
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
        exit_time_sec: parseInt(document.getElementById('exitTimeSec').value),
        enabled: document.getElementById('walletEnabled').checked,
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
        exit_time_sec: parseInt(document.getElementById('newExitTimeSec').value),
        enabled: document.getElementById('newWalletEnabled').checked
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
        document.getElementById('schemeMinBuyAmount').value = scheme.min_buy_amount;
        document.getElementById('schemeMaxBuyAmount').value = scheme.max_buy_amount;
        document.getElementById('schemeBuySolAmount').value = scheme.buy_sol_amount;
        //document.getElementById('schemeSellPercentage').value = scheme.sell_percentage;
        document.getElementById('schemeStopLossPercentage').value = scheme.stop_loss_percentage;
        document.getElementById('schemeTakeProfitPercentage').value = scheme.take_profit_percentage;
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
        min_buy_amount: parseFloat(document.getElementById('newSchemeMinBuyAmount').value),
        max_buy_amount: parseFloat(document.getElementById('newSchemeMaxBuyAmount').value),
        buy_sol_amount: parseFloat(document.getElementById('newSchemeBuySolAmount').value),
        //sell_percentage: parseFloat(document.getElementById('newSchemeSellPercentage').value),
        stop_loss_percentage: parseFloat(document.getElementById('newSchemeStopLossPercentage').value),
        take_profit_percentage: parseFloat(document.getElementById('newSchemeTakeProfitPercentage').value),
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
        min_buy_amount: parseFloat(document.getElementById('schemeMinBuyAmount').value),
        max_buy_amount: parseFloat(document.getElementById('schemeMaxBuyAmount').value),
        buy_sol_amount: parseFloat(document.getElementById('schemeBuySolAmount').value),
        //sell_percentage: parseFloat(document.getElementById('schemeSellPercentage').value),
        stop_loss_percentage: parseFloat(document.getElementById('schemeStopLossPercentage').value),
        take_profit_percentage: parseFloat(document.getElementById('schemeTakeProfitPercentage').value),
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
