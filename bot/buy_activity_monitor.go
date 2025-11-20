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

// BuyActivityTask 买单活跃度监控任务
type BuyActivityTask struct {
	MintAddress   string
	WalletAddress string
	Platform      string
	BuyPrice      float64
	BuyAmount     uint64
	BuyTime       time.Time
	LastBuyTime   time.Time // 最后一次检测到买单的时间
	Timeout       time.Duration
	SellPercent   int
	MinBuyAmount  float64 // 最小买单金额（SOL）
	Generalized   bool    // 是否泛化监控：true=持续监控，false=首次超时内有新单则永久取消
	CancelChan    chan struct{}
	Owner         solana.PublicKey
	Creator       solana.PublicKey
}

// BuyActivityMonitor 买单活跃度监控器
type BuyActivityMonitor struct {
	tasks  map[string]*BuyActivityTask // key: mintAddress
	config *utils.Config
	mutex  sync.RWMutex
}

// NewBuyActivityMonitor 创建买单活跃度监控器
func NewBuyActivityMonitor(config *utils.Config) *BuyActivityMonitor {
	return &BuyActivityMonitor{
		tasks:  make(map[string]*BuyActivityTask),
		config: config,
	}
}

// StartMonitoring 启动买单活跃度监控
func (bam *BuyActivityMonitor) StartMonitoring(task *BuyActivityTask) {
	enable := global.GetConfig().BuyActivityMonitor.Enable
	if !enable {
		logger.Infof("买单活跃度监控未启用，跳过监控: %s", task.MintAddress)
		return
	}

	bam.mutex.Lock()
	// 检查是否已存在监控任务
	if existingTask, exists := bam.tasks[task.MintAddress]; exists {
		logger.Warnf("代币 %s 已存在买单活跃度监控任务，取消旧任务", task.MintAddress)
		// 安全地关闭 channel：使用 select 避免重复关闭导致 panic
		select {
		case <-existingTask.CancelChan:
			// channel 已经关闭，不做任何操作
		default:
			close(existingTask.CancelChan)
		}
	}

	bam.tasks[task.MintAddress] = task
	bam.mutex.Unlock()

	logger.Infof("🔍 启动买单活跃度监控: 代币=%s, 超时=%ds, 卖出比例=%d%%, 最小买单=%.4f SOL",
		task.MintAddress, int(task.Timeout.Seconds()), task.SellPercent, task.MinBuyAmount)

	// 启动监控goroutine
	go bam.monitorTask(task)
}

// monitorTask 监控单个任务
func (bam *BuyActivityMonitor) monitorTask(task *BuyActivityTask) {
	checkInterval := global.GetConfig().BuyActivityMonitor.CheckInterval
	ticker := time.NewTicker(time.Duration(checkInterval) * time.Millisecond)
	defer ticker.Stop()

	// 初始化最后买单时间为当前时间（bot自己的买入时间）
	task.LastBuyTime = task.BuyTime
	startTime := time.Now()

	for {
		select {
		case <-task.CancelChan:
			logger.Infof("买单活跃度监控已取消: %s", task.MintAddress)
			bam.removeTask(task.MintAddress)
			return

		case <-ticker.C:
			// 检查是否有新买单
			if bam.hasNewBuyOrder(task) {
				// 有新买单
				task.LastBuyTime = time.Now()

				if !task.Generalized {
					// 非泛化模式：首次超时内检测到新买单，永久取消监控
					elapsedTime := time.Since(startTime)
					if elapsedTime < task.Timeout {
						logger.Infof("✅ [非泛化模式] 首次超时内检测到新买单: 代币=%s, 经过%.1fs, 永久取消监控",
							task.MintAddress, elapsedTime.Seconds())
						bam.removeTask(task.MintAddress)
						return
					}
				}

				// 泛化模式或非泛化模式且已超过首次超时：重置计时器
				logger.Infof("✅ 检测到新买单: 代币=%s, 重置监控计时器", task.MintAddress)
				startTime = time.Now() // 重置开始时间
				continue
			}

			// 检查是否超时
			elapsedTime := time.Since(startTime)
			if elapsedTime >= task.Timeout {
				logger.Warnf("⏰ 买单活跃度超时: 代币=%s, 已等待%.1fs无新买单",
					task.MintAddress, elapsedTime.Seconds())
				// 先同步移除任务，避免与 CancelMonitoring 竞态
				bam.removeTask(task.MintAddress)
				// 异步执行卖出操作，避免阻塞监控循环
				go bam.handleTimeout(task)
				return
			}

			logger.Infof("买单活跃度检查: 代币=%s, 已等待%.1fs, 超时阈值=%.1fs",
				task.MintAddress, elapsedTime.Seconds(), task.Timeout.Seconds())
		}
	}
}

// hasNewBuyOrder 检查是否有新买单
func (bam *BuyActivityMonitor) hasNewBuyOrder(task *BuyActivityTask) bool {
	// 只支持pumpfun平台
	if task.Platform != "pumpfun" {
		return false
	}

	tradeInfo := global.GetTradeInfo(task.MintAddress)
	if tradeInfo == nil {
		return false
	}

	// 检查是否是买单
	if !tradeInfo.IsBuy {
		return false
	}

	// 检查是否是新的买单（接收时间晚于上次记录的买单时间）
	if !tradeInfo.ReceivedAt.After(task.LastBuyTime) {
		return false
	}

	// 检查买单金额是否达到最小要求
	buyAmountSOL := float64(tradeInfo.SolAmount) / 1e9
	if buyAmountSOL < task.MinBuyAmount {
		logger.Debugf("检测到买单但金额过小: 代币=%s, 金额=%.4f SOL < 最小要求%.4f SOL",
			task.MintAddress, buyAmountSOL, task.MinBuyAmount)
		return false
	}

	// 过滤掉bot自己的买单
	botWallet := global.GetConfig().TradeWalletKey.PublicKey()
	if tradeInfo.UserAddress == botWallet.String() {
		logger.Debugf("过滤掉bot自己的买单: 代币=%s", task.MintAddress)
		return false
	}

	logger.Infof("💰 检测到有效新买单: 代币=%s, 用户=%s, 金额=%.4f SOL, 时间=%s",
		task.MintAddress, tradeInfo.UserAddress[:8]+"...", buyAmountSOL,
		tradeInfo.ReceivedAt.Format("15:04:05.000"))

	return true
}

// handleTimeout 处理超时情况
func (bam *BuyActivityMonitor) handleTimeout(task *BuyActivityTask) {
	logger.Warnf("⚠️ 买单活跃度不足: 代币=%s, %ds内无新买单，执行卖出%d%%",
		task.MintAddress, int(task.Timeout.Seconds()), task.SellPercent)

	// 执行卖出
	bam.executeSell(task)
}

// executeSell 执行卖出操作
func (bam *BuyActivityMonitor) executeSell(task *BuyActivityTask) {
	sellAmount := uint64(float64(task.BuyAmount) * float64(task.SellPercent) / 100.0)

	if sellAmount <= 10 {
		logger.Warnf("计算的卖出数量太小(%d)，跳过卖出", sellAmount)
		return
	}

	// 获取当前价格
	tradeInfo := global.GetTradeInfo(task.MintAddress)
	var currentPrice float64
	if tradeInfo != nil {
		currentPrice = tradeInfo.CurrentPrice
	} else {
		currentPrice = task.BuyPrice
		logger.Warnf("无法获取当前价格，使用买入价: %.8f", task.BuyPrice)
	}

	logger.Infof("📤 准备卖出(买单活跃度不足): 代币=%s, 数量=%d (%.0f%%), 买入价=%.8f, 当前价=%.8f",
		task.MintAddress, sellAmount, float64(task.SellPercent), task.BuyPrice, currentPrice)

	// 构造TokenInfo并发送到卖出通道
	mint, err := solana.PublicKeyFromBase58(task.MintAddress)
	if err != nil {
		logger.Errorf("解析Mint地址失败: %v", err)
		return
	}

	tokenInfo := queue.TokenInfo{
		PumpVersion:  tradeInfo.PumpVersion,
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
		logger.Infof("✅ 已发送卖出请求到通道(买单活跃度): 代币=%s, 数量=%d", task.MintAddress, sellAmount)
	default:
		logger.Errorf("❌ 卖出通道已满，无法发送卖出请求: %s", task.MintAddress)
	}
}

// CancelMonitoring 取消监控
func (bam *BuyActivityMonitor) CancelMonitoring(mintAddress string) {
	bam.mutex.Lock()
	defer bam.mutex.Unlock()

	if task, exists := bam.tasks[mintAddress]; exists {
		// 安全地关闭 channel：使用 select 避免重复关闭导致 panic
		select {
		case <-task.CancelChan:
			// channel 已经关闭，不做任何操作
		default:
			close(task.CancelChan)
		}
		delete(bam.tasks, mintAddress)
		logger.Infof("已取消买单活跃度监控: %s", mintAddress)
	}
}

// removeTask 移除任务
func (bam *BuyActivityMonitor) removeTask(mintAddress string) {
	bam.mutex.Lock()
	defer bam.mutex.Unlock()
	delete(bam.tasks, mintAddress)
}

// GetTaskCount 获取当前监控任务数量
func (bam *BuyActivityMonitor) GetTaskCount() int {
	bam.mutex.RLock()
	defer bam.mutex.RUnlock()
	return len(bam.tasks)
}

// IsMonitoring 检查是否正在监控某个代币
func (bam *BuyActivityMonitor) IsMonitoring(mintAddress string) bool {
	bam.mutex.RLock()
	defer bam.mutex.RUnlock()
	_, exists := bam.tasks[mintAddress]
	return exists
}
