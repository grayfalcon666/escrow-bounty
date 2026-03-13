package db

import (
	"context"
	"log"
	"time"
)

type MockBankClient struct{}

// Transfer 实现了 BankClient 接口
func (m *MockBankClient) Transfer(ctx context.Context, fromAccount, toAccount, amount int64, idempotencyKey string) error {
	log.Printf("[Mock Simplebank] 收到转账请求:")
	log.Printf("  -> 从账户: %d", fromAccount)
	log.Printf("  -> 到账户: %d (平台担保账户)", toAccount)
	log.Printf("  -> 金额: %d", amount)
	log.Printf("  -> 幂等键 (Idempotency Key): %s", idempotencyKey)

	// 模拟网络请求的延迟
	time.Sleep(500 * time.Millisecond)

	// 这里默认返回 nil，代表扣款成功。
	// 可以在这里故意 return fmt.Errorf("余额不足") 来测试 Saga 分布式事务是否能正确回滚状态为 FAILED
	log.Printf("[Mock Simplebank] 扣款成功！")
	return nil
}
