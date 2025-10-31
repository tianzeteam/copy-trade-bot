package bot

import (
	"sync"
	"time"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/utils"

	"github.com/gagliardetto/solana-go"
)

// ProfitTargetTask 盈利目标监控任务
type ProfitTargetTask struct {
	MintAddress        string
	WalletAddress      string
	Platform           string
	BuyPrice           float64
	BuyAmount          uint64
	BuyTime            time.Time
	TargetProfit       float64 // 目标盈利百分比
	Timeout            time.Duration
	PartialSellPercent int
	FullExitOnTimeout  bool
	CancelChan         chan struct{}
	Owner              solana.PublicKey
	Creator            solana.PublicKey
}

// ProfitTargetMonitor 盈利目标监控器
type ProfitTargetMonitor struct {
	tasks  map[string]*ProfitTargetTask // key: mintAddress
	config *utils.ProfitTargetMonitorConfig
	bot    *Bot
	mutex  sync.RWMutex
}

// NewProfitTargetMonitor 创建盈利目标监控器
func NewProfitTargetMonitor(config *utils.ProfitTargetMonitorConfig, bot *Bot) *ProfitTargetMonitor {
	return &ProfitTargetMonitor{
		tasks:  make(map[string]*ProfitTargetTask),
		config: config,
		bot:    bot,
	}
}

// StartMonitoring 启动盈利目标监控
func (pm *ProfitTargetMonitor) StartMonitoring(task *ProfitTargetTask) {
	if !pm.config.Enable {
		logger.Debugf("盈利目标监控未启用，跳过监控: %s", task.MintAddress)
		return
	}

	pm.mutex.Lock()
	// 检查是否已存在监控任务
	if existingTask, exists := pm.tasks[task.MintAddress]; exists {
		logger.Warnf("代币 %s 已存在监控任务，取消旧任务", task.MintAddress)
		close(existingTask.CancelChan)
	}

	pm.tasks[task.MintAddress] = task
	pm.mutex.Unlock()

	logger.Infof("启动盈利目标监控: 代币=%s, 买入价=%.8f, 目标盈利=%.2f%%, 超时=%ds",
		task.MintAddress, task.BuyPrice, task.TargetProfit, int(task.Timeout.Seconds()))

	// 启动监控goroutine
	go pm.monitorTask(task)
}

// monitorTask 监控单个任务
func (pm *ProfitTargetMonitor) monitorTask(task *ProfitTargetTask) {
	checkInterval := time.Duration(pm.config.CheckInterval) * time.Second
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(task.Timeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-task.CancelChan:
			logger.Infof("盈利目标监控已取消: %s", task.MintAddress)
			pm.removeTask(task.MintAddress)
			return

		case <-timeoutTimer.C:
			logger.Infof("盈利目标监控超时: %s, 检查是否达标", task.MintAddress)
			pm.handleTimeout(task)
			pm.removeTask(task.MintAddress)
			return

		case <-ticker.C:
			// 定期检查盈利是否达标
			if pm.checkAndHandleProfit(task) {
				// 达标，取消监控
				pm.removeTask(task.MintAddress)
				return
			}
		}
	}
}

// checkAndHandleProfit 检查盈利并处理
func (pm *ProfitTargetMonitor) checkAndHandleProfit(task *ProfitTargetTask) bool {
	currentPrice := pm.getCurrentPrice(task.MintAddress, task.Platform)
	if currentPrice <= 0 {
		logger.Warnf("无法获取代币 %s 的当前价格", task.MintAddress)
		return false
	}

	profitPercent := (currentPrice - task.BuyPrice) / task.BuyPrice * 100
	logger.Debugf("盈利检查: 代币=%s, 买入价=%.8f, 当前价=%.8f, 盈利=%.2f%%, 目标=%.2f%%",
		task.MintAddress, task.BuyPrice, currentPrice, profitPercent, task.TargetProfit)

	if profitPercent >= task.TargetProfit {
		logger.Infof("✅ 盈利达标: 代币=%s, 盈利=%.2f%% >= 目标%.2f%%, 取消监控，进入正常多级卖出",
			task.MintAddress, profitPercent, task.TargetProfit)
		return true
	}

	return false
}

// handleTimeout 处理超时情况
func (pm *ProfitTargetMonitor) handleTimeout(task *ProfitTargetTask) {
	currentPrice := pm.getCurrentPrice(task.MintAddress, task.Platform)
	if currentPrice <= 0 {
		logger.Errorf("超时处理失败: 无法获取代币 %s 的当前价格", task.MintAddress)
		return
	}

	profitPercent := (currentPrice - task.BuyPrice) / task.BuyPrice * 100

	if profitPercent >= task.TargetProfit {
		logger.Infof("✅ 超时检查: 代币=%s 已达标(%.2f%%), 无需处理", task.MintAddress, profitPercent)
		return
	}

	logger.Warnf("⚠️ 盈利未达标: 代币=%s, 当前盈利=%.2f%% < 目标%.2f%%",
		task.MintAddress, profitPercent, task.TargetProfit)

	// 决定卖出比例
	var sellPercent int
	if task.FullExitOnTimeout {
		sellPercent = 100
		logger.Infof("执行全部清仓: 代币=%s, 卖出100%%", task.MintAddress)
	} else {
		sellPercent = task.PartialSellPercent
		logger.Infof("执行部分卖出: 代币=%s, 卖出%d%%", task.MintAddress, sellPercent)
	}

	// 执行卖出
	pm.executeSell(task, sellPercent, currentPrice)
}

// executeSell 执行卖出操作
func (pm *ProfitTargetMonitor) executeSell(task *ProfitTargetTask, sellPercent int, currentPrice float64) {
	sellAmount := uint64(float64(task.BuyAmount) * float64(sellPercent) / 100.0)

	if sellAmount <= 10 {
		logger.Warnf("计算的卖出数量太小(%d)，跳过卖出", sellAmount)
		return
	}

	logger.Infof("准备卖出: 代币=%s, 数量=%d (%.0f%%), 当前价=%.8f",
		task.MintAddress, sellAmount, float64(sellPercent), currentPrice)

	// 构造TokenInfo并发送到卖出通道
	mint, err := solana.PublicKeyFromBase58(task.MintAddress)
	if err != nil {
		logger.Errorf("解析Mint地址失败: %v", err)
		return
	}

	tokenInfo := queue.TokenInfo{
		Mint:         mint,
		Amount:       sellAmount,
		Owner:        task.Owner,
		User:         task.Owner,
		Platform:     task.Platform,
		EntryPrice:   task.BuyPrice,
		CurrentPrice: currentPrice,
		Creator:      task.Creator,
	}

	// 发送到卖出通道
	select {
	case queue.WaitingSellChan <- tokenInfo:
		logger.Infof("已发送卖出请求到通道: 代币=%s, 数量=%d", task.MintAddress, sellAmount)
	default:
		logger.Errorf("卖出通道已满，无法发送卖出请求: %s", task.MintAddress)
	}
}

// getCurrentPrice 获取当前价格
func (pm *ProfitTargetMonitor) getCurrentPrice(mintAddress string, platform string) float64 {
	switch platform {
	case "pumpfun":
		tradeInfo := global.GetTradeInfo(mintAddress)
		if tradeInfo != nil {
			return tradeInfo.CurrentPrice
		}

	case "pumpamm":
		tradeInfo := global.GetAmmTradeInfo(mintAddress)
		if tradeInfo != nil {
			return tradeInfo.CurrentPrice
		}

	case "raydiumV4":
		tradeInfo := global.GetAmmRaydiuV4TradeInfo(mintAddress)
		if tradeInfo != nil {
			return tradeInfo.CurrentPrice
		}

	case "launchpad":
		tradeInfo := global.GetLaunchpadTradeInfo(mintAddress)
		if tradeInfo != nil {
			return tradeInfo.CurrentPrice
		}

	case "cpmm":
		tradeInfo := global.GetCpmmTradeInfo(mintAddress)
		if tradeInfo != nil {
			return tradeInfo.CurrentPrice
		}

	case "meteora-dbc":
		tradeInfo := global.GetDBCTradeInfo(mintAddress)
		if tradeInfo != nil {
			return tradeInfo.CurrentPrice
		}

	default:
		logger.Warnf("未知平台: %s", platform)
	}

	return 0
}

// CancelMonitoring 取消监控
func (pm *ProfitTargetMonitor) CancelMonitoring(mintAddress string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if task, exists := pm.tasks[mintAddress]; exists {
		close(task.CancelChan)
		delete(pm.tasks, mintAddress)
		logger.Infof("已取消盈利目标监控: %s", mintAddress)
	}
}

// removeTask 移除任务
func (pm *ProfitTargetMonitor) removeTask(mintAddress string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	delete(pm.tasks, mintAddress)
}

// GetTaskCount 获取当前监控任务数量
func (pm *ProfitTargetMonitor) GetTaskCount() int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return len(pm.tasks)
}

// IsMonitoring 检查是否正在监控某个代币
func (pm *ProfitTargetMonitor) IsMonitoring(mintAddress string) bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	_, exists := pm.tasks[mintAddress]
	return exists
}
