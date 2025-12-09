package shredstream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"copytrade/logger"

	"github.com/mr-tron/base58"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client ShredStream gRPC 客户端
type Client struct {
	endpoint string
	conn     *grpc.ClientConn

	ctx    context.Context
	cancel context.CancelFunc

	mu        sync.RWMutex
	isRunning bool

	// 配置
	reconnectInterval time.Duration
	maxRetries        int

	// 事件通道
	EventChan chan *ShredEvent
}

// NewClient 创建 ShredStream 客户端
func NewClient(config ShredStreamConfig) (*Client, error) {
	if config.Endpoint == "" {
		return nil, fmt.Errorf("shredstream endpoint is required")
	}

	reconnectInterval := time.Duration(config.ReconnectInterval) * time.Second
	if reconnectInterval == 0 {
		reconnectInterval = 5 * time.Second
	}

	maxRetries := config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 10
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		endpoint:          config.Endpoint,
		ctx:               ctx,
		cancel:            cancel,
		reconnectInterval: reconnectInterval,
		maxRetries:        maxRetries,
		EventChan:         make(chan *ShredEvent, 1000),
	}, nil
}

// Start 启动 ShredStream 订阅
func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		return fmt.Errorf("shredstream client is already running")
	}

	// 建立 gRPC 连接
	conn, err := grpc.NewClient(
		c.endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to shredstream: %w", err)
	}
	c.conn = conn
	c.isRunning = true

	// 启动订阅处理
	go c.subscribeLoop()

	logger.Infof("ShredStream client started, endpoint: %s", c.endpoint)
	return nil
}

// Stop 停止 ShredStream 客户端
func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		return
	}

	c.cancel()
	c.isRunning = false

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	close(c.EventChan)
	logger.Infof("ShredStream client stopped")
}

// IsRunning 检查客户端是否正在运行
func (c *Client) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isRunning
}

// subscribeLoop 订阅循环（带重连机制）
func (c *Client) subscribeLoop() {
	retryCount := 0

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		err := c.subscribe()
		if err != nil {
			logger.Errorf("ShredStream subscription error: %v", err)

			retryCount++
			if retryCount > c.maxRetries {
				logger.Errorf("ShredStream max retries exceeded, stopping client")
				c.Stop()
				return
			}

			logger.Infof("ShredStream reconnecting in %v (attempt %d/%d)",
				c.reconnectInterval, retryCount, c.maxRetries)
			time.Sleep(c.reconnectInterval)
			continue
		}

		// 成功后重置重试计数
		retryCount = 0
	}
}

// subscribe 执行实际的订阅逻辑
// 注意: 这是一个简化的实现，实际需要根据 ShredStream 服务的 proto 定义来实现
func (c *Client) subscribe() error {
	// TODO: 实现实际的 gRPC 订阅逻辑
	// 这里需要根据实际的 ShredStream proto 定义来实现
	//
	// 示例伪代码:
	// stream, err := c.client.SubscribeEntries(c.ctx, &pb.SubscribeEntriesRequest{})
	// if err != nil {
	//     return err
	// }
	//
	// for {
	//     resp, err := stream.Recv()
	//     if err != nil {
	//         return err
	//     }
	//     c.processEntries(resp.Slot, resp.Entries)
	// }

	logger.Warnf("ShredStream subscribe() is a placeholder - implement with actual proto")

	// 临时: 等待上下文取消
	<-c.ctx.Done()
	return c.ctx.Err()
}

// processEntries 处理接收到的 Entry 数据
func (c *Client) processEntries(slot uint64, entriesData []byte) {
	entries, err := DecodeEntries(entriesData)
	if err != nil {
		logger.Errorf("Failed to decode entries for slot %d: %v", slot, err)
		return
	}

	receivedAt := time.Now()

	for _, entry := range entries {
		for i := range entry.Transactions {
			tx := &entry.Transactions[i]

			// 获取交易签名
			if len(tx.Signatures) == 0 {
				continue
			}
			signature := base58.Encode(tx.Signatures[0][:])

			event := &ShredEvent{
				Source:      "shredstream",
				Slot:        slot,
				Signature:   signature,
				ReceivedAt:  receivedAt,
				Transaction: tx,
			}

			// 非阻塞发送
			select {
			case c.EventChan <- event:
			default:
				logger.Warnf("ShredStream event channel full, dropping event: %s", signature)
			}
		}
	}
}

// ProcessRawEntries 外部调用的入口，用于处理原始 Entry 数据
// 当有实际的 ShredStream 数据源时调用此方法
func (c *Client) ProcessRawEntries(slot uint64, entriesData []byte) {
	c.processEntries(slot, entriesData)
}
