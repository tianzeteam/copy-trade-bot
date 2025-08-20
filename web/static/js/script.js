// 全局变量
let currentWallet = null;
let wallets = [];
let tradeSettings = {};

// DOM 元素
const walletSelect = document.getElementById('walletSelect');
const walletInfo = document.getElementById('walletInfo');
const noWalletAlert = document.getElementById('noWalletAlert');
const walletName = document.getElementById('walletName');
const walletAddress = document.getElementById('walletAddress');
const walletType = document.getElementById('walletType');
const takeProfitPct = document.getElementById('takeProfitPct');
const stopLossPct = document.getElementById('stopLossPct');

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
const newWalletEnabled = document.getElementById('newWalletEnabled');
const addWalletModal = new bootstrap.Modal(document.getElementById('addWalletModal'));

// 交易设置元素
const pumpTradeEnable = document.getElementById('pumpTradeEnable');
const pumpAmmTradeEnable = document.getElementById('pumpAmmTradeEnable');
const raydiumTradeEnable = document.getElementById('raydiumTradeEnable');
const minPoolSize = document.getElementById('minPoolSize');
const maxPoolSize = document.getElementById('maxPoolSize');
const minFollowTriggerAmount = document.getElementById('minFollowTriggerAmount');
const maxFollowTriggerAmount = document.getElementById('maxFollowTriggerAmount');
const buySlippage = document.getElementById('buySlippage');
const buyPriorityFee = document.getElementById('buyPriorityFee');
const buyTipFee = document.getElementById('buyTipFee');
const buySolAmount = document.getElementById('buySolAmount');
const sellSlippage = document.getElementById('sellSlippage');
const sellPriorityFee = document.getElementById('sellPriorityFee');
const sellTipFee = document.getElementById('sellTipFee');
const sellDelayTime = document.getElementById('sellDelayTime');
const saveTradeSettingsBtn = document.getElementById('saveTradeSettingsBtn');

// 页面加载时获取钱包列表和交易设置
document.addEventListener('DOMContentLoaded', async () => {
    await Promise.all([
        loadWallets(),
        loadTradeSettings()
    ]);
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

// 保存交易设置按钮点击事件
saveTradeSettingsBtn.addEventListener('click', async () => {
    await saveTradeSettings();
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

// 加载交易设置
async function loadTradeSettings() {
    try {
        // 添加时间戳参数防止缓存
        const timestamp = new Date().getTime();
        const response = await fetch(`/api/trade-settings?t=${timestamp}`);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        tradeSettings = await response.json();
        console.log('加载的交易设置:', tradeSettings);
        
        // 填充表单数据
        pumpTradeEnable.checked = tradeSettings.pump_trade_enable;
        pumpAmmTradeEnable.checked = tradeSettings.pump_amm_trade_enable;
        raydiumTradeEnable.checked = tradeSettings.raydium_trade_enable;
        minPoolSize.value = tradeSettings.min_pool_size;
        maxPoolSize.value = tradeSettings.max_pool_size;
        minFollowTriggerAmount.value = tradeSettings.min_follow_trigger_amount;
        maxFollowTriggerAmount.value = tradeSettings.max_follow_trigger_amount;
        buySlippage.value = tradeSettings.buy_slippage;
        buyPriorityFee.value = tradeSettings.buy_priority_fee;
        buyTipFee.value = tradeSettings.buy_tip_fee;
        buySolAmount.value = tradeSettings.buy_sol_amount;
        sellSlippage.value = tradeSettings.sell_slippage;
        sellPriorityFee.value = tradeSettings.sell_priority_fee;
        sellTipFee.value = tradeSettings.sell_tip_fee;
        sellDelayTime.value = tradeSettings.sell_delay_time || 0;
    } catch (error) {
        console.error('获取交易设置失败:', error);
        showAlert(errorAlert, `加载交易设置失败: ${error.message}`);
    }
}

// 保存交易设置
async function saveTradeSettings() {
    try {
        const updates = {
            pump_trade_enable: pumpTradeEnable.checked,
            pump_amm_trade_enable: pumpAmmTradeEnable.checked,
            raydium_trade_enable: raydiumTradeEnable.checked,
            min_pool_size: parseFloat(minPoolSize.value),
            max_pool_size: parseFloat(maxPoolSize.value),
            min_follow_trigger_amount: parseFloat(minFollowTriggerAmount.value),
            max_follow_trigger_amount: parseFloat(maxFollowTriggerAmount.value),
            buy_slippage: parseFloat(buySlippage.value),
            buy_priority_fee: parseFloat(buyPriorityFee.value),
            buy_tip_fee: parseFloat(buyTipFee.value),
            buy_sol_amount: parseFloat(buySolAmount.value),
            sell_slippage: parseFloat(sellSlippage.value),
            sell_priority_fee: parseFloat(sellPriorityFee.value),
            sell_tip_fee: parseFloat(sellTipFee.value),
            sell_delay_time: parseInt(sellDelayTime.value) || 0
        };
        
        const response = await fetch('/api/trade-settings', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(updates)
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        // 显示成功消息
        showAlert(successAlert, '交易设置保存成功！');
        
        // 重新加载交易设置
        await loadTradeSettings();
    } catch (error) {
        console.error('保存交易设置失败:', error);
        showAlert(errorAlert, `保存交易设置失败: ${error.message}`);
    }
}
