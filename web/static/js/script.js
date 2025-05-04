// 全局变量
let currentWallet = null;
let wallets = [];

// DOM 元素
const walletSelect = document.getElementById('walletSelect');
const walletInfo = document.getElementById('walletInfo');
const noWalletAlert = document.getElementById('noWalletAlert');
const walletName = document.getElementById('walletName');
const walletAddress = document.getElementById('walletAddress');
const walletType = document.getElementById('walletType');
const takeProfitPct = document.getElementById('takeProfitPct');
const stopLossPct = document.getElementById('stopLossPct');
const exitTimeSec = document.getElementById('exitTimeSec');
const walletEnabled = document.getElementById('walletEnabled');
const deleteWalletBtn = document.getElementById('deleteWalletBtn');
const updateWalletBtn = document.getElementById('updateWalletBtn');
const saveNewWalletBtn = document.getElementById('saveNewWalletBtn');
const successAlert = document.getElementById('successAlert');
const errorAlert = document.getElementById('errorAlert');

// 添加钱包表单元素
const newWalletAddress = document.getElementById('newWalletAddress');
const newWalletName = document.getElementById('newWalletName');
const newWalletType = document.getElementById('newWalletType');
const newTakeProfitPct = document.getElementById('newTakeProfitPct');
const newStopLossPct = document.getElementById('newStopLossPct');
const newExitTimeSec = document.getElementById('newExitTimeSec');
const newWalletEnabled = document.getElementById('newWalletEnabled');
const addWalletModal = new bootstrap.Modal(document.getElementById('addWalletModal'));

// 页面加载时获取钱包列表
document.addEventListener('DOMContentLoaded', async () => {
    await loadWallets();
});

// 加载钱包列表
async function loadWallets() {
    try {
        // 显示加载中提示
        const loadingMessage = document.createElement('div');
        loadingMessage.className = 'alert alert-info mt-3';
        loadingMessage.textContent = '正在加载钱包列表...';
        document.querySelector('.container').appendChild(loadingMessage);
        
        // 添加时间戳参数防止缓存
        const timestamp = new Date().getTime();
        const response = await fetch(`/api/wallets?t=${timestamp}`);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        wallets = await response.json();
        
        // 移除加载中提示
        loadingMessage.remove();
        
        console.log('加载的钱包列表:', wallets);
        
        // 渲染钱包下拉列表
        renderWalletSelect();
    } catch (error) {
        console.error('获取钱包列表失败:', error);
        
        // 显示错误消息
        showAlert(errorAlert, `加载钱包列表失败: ${error.message}`);
    }
}

// 渲染钱包下拉列表
function renderWalletSelect() {
    // 清空下拉列表
    walletSelect.innerHTML = '';
    
    if (wallets.length === 0) {
        // 显示无钱包提示
        walletInfo.classList.add('d-none');
        noWalletAlert.classList.remove('d-none');
        return;
    }
    
    // 隐藏无钱包提示
    noWalletAlert.classList.add('d-none');
    
    // 添加钱包选项
    wallets.forEach(wallet => {
        const option = document.createElement('option');
        option.value = wallet.address;
        option.textContent = `${wallet.name} (${wallet.address.substring(0, 8)}...${wallet.address.substring(wallet.address.length - 4)})`;
        walletSelect.appendChild(option);
    });
    
    // 默认选择第一个钱包
    if (wallets.length > 0) {
        walletSelect.value = wallets[0].address;
        displayWalletInfo(wallets[0]);
    }
}

// 显示钱包信息
function displayWalletInfo(wallet) {
    currentWallet = wallet;
    
    // 显示钱包信息卡片
    walletInfo.classList.remove('d-none');
    
    // 填充钱包信息
    walletName.textContent = wallet.name;
    walletAddress.textContent = wallet.address;
    walletType.textContent = wallet.type === 'high_frequency' ? '高频交易' : '低频交易';
    takeProfitPct.value = wallet.take_profit_pct;
    stopLossPct.value = wallet.stop_loss_pct;
    exitTimeSec.value = wallet.exit_time_sec;
    walletEnabled.checked = wallet.enabled;
}

// 钱包选择变更事件
walletSelect.addEventListener('change', () => {
    const selectedAddress = walletSelect.value;
    const selectedWallet = wallets.find(w => w.address === selectedAddress);
    
    if (selectedWallet) {
        displayWalletInfo(selectedWallet);
    }
});

// 更新钱包按钮点击事件
updateWalletBtn.addEventListener('click', async () => {
    if (!currentWallet) return;
    
    try {
        const updatedWallet = {
            address: currentWallet.address,
            name: currentWallet.name,
            type: currentWallet.type,
            take_profit_pct: parseInt(takeProfitPct.value),
            stop_loss_pct: parseInt(stopLossPct.value),
            exit_time_sec: parseInt(exitTimeSec.value),
            enabled: walletEnabled.checked,
            timestamp: new Date().getTime() // 添加时间戳确保每次请求唯一
        };
        
        const response = await fetch(`/api/wallets/${currentWallet.address}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(updatedWallet)
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        // 更新本地钱包列表
        const index = wallets.findIndex(w => w.address === currentWallet.address);
        if (index !== -1) {
            wallets[index] = updatedWallet;
        }
        
        // 显示成功消息
        showAlert(successAlert, '钱包更新成功！');
        
        // 重新加载钱包列表
        await loadWallets();
    } catch (error) {
        console.error('更新钱包失败:', error);
        showAlert(errorAlert, `更新钱包失败: ${error.message}`);
    }
});

// 删除钱包按钮点击事件
deleteWalletBtn.addEventListener('click', async () => {
    if (!currentWallet) return;
    
    if (!confirm(`确定要删除钱包 ${currentWallet.name} (${currentWallet.address}) 吗？`)) {
        return;
    }
    
    try {
        const response = await fetch(`/api/wallets/${currentWallet.address}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        // 显示成功消息
        showAlert(successAlert, '钱包删除成功！');
        
        // 重新加载钱包列表
        await loadWallets();
    } catch (error) {
        console.error('删除钱包失败:', error);
        showAlert(errorAlert, `删除钱包失败: ${error.message}`);
    }
});

// 保存新钱包按钮点击事件
saveNewWalletBtn.addEventListener('click', async () => {
    // 表单验证
    if (!validateNewWalletForm()) {
        return;
    }
    
    try {
        const newWallet = {
            address: newWalletAddress.value.trim(),
            name: newWalletName.value.trim(),
            type: newWalletType.value,
            take_profit_pct: parseInt(newTakeProfitPct.value),
            stop_loss_pct: parseInt(newStopLossPct.value),
            exit_time_sec: parseInt(newExitTimeSec.value),
            enabled: newWalletEnabled.checked,
            timestamp: new Date().getTime() // 添加时间戳确保每次请求唯一
        };
        
        const response = await fetch('/api/wallets', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(newWallet)
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        // 关闭模态框
        addWalletModal.hide();
        
        // 重置表单
        document.getElementById('addWalletForm').reset();
        
        // 显示成功消息
        showAlert(successAlert, '钱包添加成功！');
        
        // 重新加载钱包列表
        await loadWallets();
    } catch (error) {
        console.error('添加钱包失败:', error);
        showAlert(errorAlert, `添加钱包失败: ${error.message}`);
    }
});

// 验证新钱包表单
function validateNewWalletForm() {
    // 验证钱包地址
    const address = newWalletAddress.value.trim();
    if (!address) {
        showAlert(errorAlert, '请输入钱包地址');
        return false;
    }
    
    // 验证钱包地址格式 (简单验证，Solana地址为44个字符的base58编码)
    if (!/^[1-9A-HJ-NP-Za-km-z]{43,44}$/.test(address)) {
        showAlert(errorAlert, '请输入有效的Solana钱包地址');
        return false;
    }
    
    // 验证钱包名称
    const name = newWalletName.value.trim();
    if (!name) {
        showAlert(errorAlert, '请输入钱包名称');
        return false;
    }
    
    // 验证止盈百分比
    const takeProfitPct = parseInt(newTakeProfitPct.value);
    if (isNaN(takeProfitPct) || takeProfitPct <= 0) {
        showAlert(errorAlert, '请输入有效的止盈百分比');
        return false;
    }
    
    // 验证止损百分比
    const stopLossPct = parseInt(newStopLossPct.value);
    if (isNaN(stopLossPct) || stopLossPct <= 0) {
        showAlert(errorAlert, '请输入有效的止损百分比');
        return false;
    }
    
    // 验证持仓时间
    const exitTimeSec = parseInt(newExitTimeSec.value);
    if (isNaN(exitTimeSec) || exitTimeSec <= 0) {
        showAlert(errorAlert, '请输入有效的持仓时间');
        return false;
    }
    
    return true;
}

// 显示提示消息
function showAlert(alertElement, message) {
    alertElement.textContent = message;
    alertElement.classList.remove('d-none');
    
    // 3秒后隐藏提示
    setTimeout(() => {
        alertElement.classList.add('d-none');
    }, 3000);
}
