package bufutil

import (
	"context"
	"log"
	"sync"
	"time"
)

type BatchWriter[T any] interface {
	// BatchWrite 由写入者提供，将内容批量写入，并返回成功写入的数量和错误
	// 如果返回了错误，且数量不等于缓冲区队列长度，缓冲区会将剩下的消息重新放进队列下次重试。
	// 因此，只有当你确定这条消息写入失败后需要下次重试时，返回的数量才 != len(msgs)。
	// 如果某条消息写入失败，但业务上可以接受，建议在BatchWrite里面打条日志，继续处理
	// 下一条消息，最终依旧返回len(msgs), nil。 如果你遇到写入失败但不想丢弃消息，可以
	// return 成功的数量, err
	BatchWrite(msgs []T) (int, error)
}

type AutoFlushBuffer[T any] struct {
	buf           []T            // 缓冲区
	flushDuration time.Duration  // 定时刷入的时间间隔
	flushSize     int            // 批量刷入的阈值
	lock          sync.RWMutex   // 锁，修改缓冲区时进行保护
	writer        BatchWriter[T] // 写入的实现
}

// NewAutoFlushBuffer 创建一个缓冲区, 该缓冲区支持自动定时刷入, 支持手动刷入, 支持达到阈值批量刷入
// 该缓冲区是线程安全的, 多个协程可以并发写入, 消息会按顺序调用用户提供的BatchWrite方法进行写入
// 如果你需要并发或异步写入，只需要在BatchWrite方法中自行实现进行控制即可
func NewAutoFlushBuffer[T any](flushDuration time.Duration, flushSize int, writer BatchWriter[T]) *AutoFlushBuffer[T] {
	return &AutoFlushBuffer[T]{
		buf:           make([]T, 0, flushSize),
		flushDuration: flushDuration,
		flushSize:     flushSize,
		lock:          sync.RWMutex{},
		writer:        writer,
	}
}

// StartTimedFlusher 会根据设置的时间间隔，定期将缓冲区中的消息进行刷入
func (a *AutoFlushBuffer[T]) StartTimedFlusher(ctx context.Context) {
	ticker := time.NewTicker(a.flushDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := a.Flush(); err != nil {
				log.Printf("定时刷入时发生了错误: %v", err)
			}
		case <-ctx.Done():
			// 退出前尝试把缓冲区中的消息刷入
			_ = a.Flush()
			return
		}
	}
}

// WriteMessage 往缓冲区队列中写入一条消息
func (a *AutoFlushBuffer[T]) WriteMessage(msg T) {
	bufLen := a.writeBuf(msg)
	// 如果写入后的长度大于等于设置的阈值，则尝试刷入
	if bufLen >= a.flushSize {
		if err := a.Flush(); err != nil {
			log.Printf("写入时发生了错误: %v", err)
			return
		}
	}
}

// writeBuf 写入缓冲区并返回缓冲区的当前长度
func (a *AutoFlushBuffer[T]) writeBuf(msg T) int {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.buf = append(a.buf, msg)
	return len(a.buf)
}

// Flush 调用用户提供的处理函数，将buffer中的内容刷入
func (a *AutoFlushBuffer[T]) Flush() error {
	a.lock.Lock()
	defer a.lock.Unlock()
	if len(a.buf) > 0 {
		n, err := a.writer.BatchWrite(a.buf)
		if err != nil {
			// 原地将队列里未写完的数据拷贝到缓冲区前面
			a.buf = append(a.buf[:0], a.buf[n:]...)
			return err
		}
		a.buf = a.buf[:0]
	}
	return nil
}
