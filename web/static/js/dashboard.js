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
}

// 注册事件处理器
function registerEventHandlers() {
    // 导航链接点击事件
    document.getElementById('walletManagementLink').addEventListener('click', function(e) {
        e.preventDefault();
        showSection('walletManagement');
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
}

// 显示指定部分
function showSection(section) {
    // 隐藏所有部分
    document.getElementById('walletManagementSection').classList.add('d-none');
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
        
        // 显示钱包详情
        document.getElementById('walletInfo').classList.remove('d-none');
    })
    .catch(error => {
        console.error('加载钱包详情失败:', error);
        showError('加载钱包详情失败，请重试');
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
        name: document.getElementById('walletName').textContent,
        take_profit_pct: parseFloat(document.getElementById('takeProfitPct').value),
        stop_loss_pct: parseFloat(document.getElementById('stopLossPct').value),
        exit_time_sec: parseInt(document.getElementById('exitTimeSec').value),
        enabled: document.getElementById('walletEnabled').checked
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
        if (response.status === 401) {
            logout();
            return;
        }
        return response.json();
    })
    .then(data => {
        if (!data) return;
        
        if (data.status === 'success') {
            showSuccess('钱包设置已更新');
            loadWallets(); // 重新加载钱包列表
        } else {
            showError('更新钱包设置失败: ' + (data.error || '未知错误'));
        }
    })
    .catch(error => {
        console.error('更新钱包设置失败:', error);
        showError('更新钱包设置失败，请重试');
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
        
        if (data.status === 'success') {
            showSuccess('钱包已删除');
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
        
        if (data.status === 'success') {
            // 关闭模态框
            const modal = bootstrap.Modal.getInstance(document.getElementById('addWalletModal'));
            modal.hide();
            
            // 清空表单
            document.getElementById('addWalletForm').reset();
            
            showSuccess('钱包已添加');
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
        
        // 填充交易设置
        document.getElementById('pumpTradeEnable').checked = settings.pump_trade_enable;
        document.getElementById('pumpAmmTradeEnable').checked = settings.pump_amm_trade_enable;
        document.getElementById('raydiumTradeEnable').checked = settings.raydium_trade_enable;
        document.getElementById('sellDelayTime').value = settings.sell_delay_time;
        document.getElementById('minPoolSize').value = settings.min_pool_size;
        document.getElementById('maxPoolSize').value = settings.max_pool_size;
        document.getElementById('minFollowTriggerAmount').value = settings.min_follow_trigger_amount;
        document.getElementById('maxFollowTriggerAmount').value = settings.max_follow_trigger_amount;
        document.getElementById('buySlippage').value = settings.buy_slippage;
        document.getElementById('buyPriorityFee').value = settings.buy_priority_fee;
        document.getElementById('buyTipFee').value = settings.buy_tip_fee;
        document.getElementById('buySolAmount').value = settings.buy_sol_amount;
        document.getElementById('sellSlippage').value = settings.sell_slippage;
        document.getElementById('sellPriorityFee').value = settings.sell_priority_fee;
        document.getElementById('sellTipFee').value = settings.sell_tip_fee;
    })
    .catch(error => {
        console.error('加载交易设置失败:', error);
        showError('加载交易设置失败，请刷新页面重试');
    });
}

// 保存交易设置
function saveTradeSettings() {
    const settings = {
        pump_trade_enable: document.getElementById('pumpTradeEnable').checked,
        pump_amm_trade_enable: document.getElementById('pumpAmmTradeEnable').checked,
        raydium_trade_enable: document.getElementById('raydiumTradeEnable').checked,
        sell_delay_time: parseInt(document.getElementById('sellDelayTime').value),
        min_pool_size: parseFloat(document.getElementById('minPoolSize').value),
        max_pool_size: parseFloat(document.getElementById('maxPoolSize').value),
        min_follow_trigger_amount: parseFloat(document.getElementById('minFollowTriggerAmount').value),
        max_follow_trigger_amount: parseFloat(document.getElementById('maxFollowTriggerAmount').value),
        buy_slippage: parseFloat(document.getElementById('buySlippage').value),
        buy_priority_fee: parseFloat(document.getElementById('buyPriorityFee').value),
        buy_tip_fee: parseFloat(document.getElementById('buyTipFee').value),
        buy_sol_amount: parseFloat(document.getElementById('buySolAmount').value),
        sell_slippage: parseFloat(document.getElementById('sellSlippage').value),
        sell_priority_fee: parseFloat(document.getElementById('sellPriorityFee').value),
        sell_tip_fee: parseFloat(document.getElementById('sellTipFee').value)
    };
    
    fetch('/api/trade-settings', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${authToken}`
        },
        body: JSON.stringify(settings)
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
        
        if (data.status === 'success') {
            showSuccess('交易设置已保存');
        } else {
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
    const alert = document.getElementById('successAlert');
    alert.textContent = message;
    alert.classList.remove('d-none');
    
    // 3秒后隐藏
    setTimeout(() => {
        alert.classList.add('d-none');
    }, 3000);
}

// 显示错误消息
function showError(message) {
    const alert = document.getElementById('errorAlert');
    alert.textContent = message;
    alert.classList.remove('d-none');
    
    // 5秒后隐藏
    setTimeout(() => {
        alert.classList.add('d-none');
    }, 5000);
}
