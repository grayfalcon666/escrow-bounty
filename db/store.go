package db

import (
	"context"
	"fmt"
	"log"

	"github.com/grayfalcon666/escrow-bounty/models"
	"gorm.io/gorm"
)

type Store interface {
	CreateBounty(ctx context.Context, bounty *models.Bounty) error
	GetBountyByID(ctx context.Context, id int64) (*models.Bounty, error)
	ListBounties(ctx context.Context, status models.BountyStatus, limit, offset int) ([]models.Bounty, error)
	UpdateBountyStatus(ctx context.Context, id int64, status models.BountyStatus) error
	CreateApplication(ctx context.Context, app *models.BountyApplication) error
	UpdateApplicationStatus(ctx context.Context, applicationID int64, status models.ApplicationStatus) error
	PublishBounty(ctx context.Context, bounty *models.Bounty, bankClient BankClient, employerAccountID, platformEscrowAccountID int64) error
	AcceptBounty(ctx context.Context, bountyID, hunter_account_id int64, hunterUsername string) (*models.BountyApplication, error)
	ConfirmHunter(ctx context.Context, bountyID int64, applicationID int64, employerUsername string) error
	CompleteBounty(ctx context.Context, bountyID int64, employerUsername string, bankClient BankClient, platformAccountID int64) error
	CancelBounty(ctx context.Context, bountyID int64, employerUsername string, bankClient BankClient, platformAccountID int64) error
}

type SQLStore struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) Store {
	return &SQLStore{
		db: db,
	}
}

// ==========================================
// Bounties (悬赏操作)
// ==========================================
func (s *SQLStore) CreateBounty(ctx context.Context, bounty *models.Bounty) error {
	return s.db.WithContext(ctx).Create(bounty).Error
}

// 根据 ID 获取单个悬赏详情，并使用 Preload 预加载关联的申请列表
func (s *SQLStore) GetBountyByID(ctx context.Context, id int64) (*models.Bounty, error) {
	var bounty models.Bounty
	err := s.db.WithContext(ctx).Preload("Applications").First(&bounty, id).Error
	if err != nil {
		return nil, err
	}
	return &bounty, nil
}

// 分页获取悬赏列表，支持按状态过滤
func (s *SQLStore) ListBounties(ctx context.Context, status models.BountyStatus, limit, offset int) ([]models.Bounty, error) {
	var bounties []models.Bounty
	query := s.db.WithContext(ctx).Limit(limit).Offset(offset)

	// 如果传入了状态参数，则动态拼接 WHERE 条件
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Find(&bounties).Error
	return bounties, err
}

// UpdateBountyStatus 更新悬赏的状态
func (s *SQLStore) UpdateBountyStatus(ctx context.Context, id int64, status models.BountyStatus) error {
	return s.db.WithContext(ctx).
		Model(&models.Bounty{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// ==========================================
// Bounty Applications (接单申请操作)
// ==========================================

func (s *SQLStore) CreateApplication(ctx context.Context, app *models.BountyApplication) error {
	return s.db.WithContext(ctx).Create(app).Error
}

func (s *SQLStore) UpdateApplicationStatus(ctx context.Context, applicationID int64, status models.ApplicationStatus) error {
	return s.db.WithContext(ctx).
		Model(&models.BountyApplication{}).
		Where("id = ?", applicationID).
		Update("status", status).Error
}

// ==========================================
// rpc操作与业务逻辑
// ==========================================

type BankClient interface {
	// Transfer 发起转账 携带 idempotencyKey 防重放
	Transfer(ctx context.Context, fromAccount, toAccount int64, amount int64, idempotencyKey string) error
}

func (s *SQLStore) PublishBounty(ctx context.Context, bounty *models.Bounty, bankClient BankClient, employerAccountID, platformEscrowAccountID int64) error {

	bounty.Status = models.BountyStatusPaying
	bounty.EmployerAccountID = employerAccountID
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(bounty).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("本地创建悬赏失败: %w", err)
	}

	idempotencyKey := fmt.Sprintf("publish_bounty_%d", bounty.ID)

	rpcErr := bankClient.Transfer(ctx, employerAccountID, platformEscrowAccountID, bounty.RewardAmount, idempotencyKey)

	if rpcErr != nil {
		// 分布式系统的部分失败处理 (Partial Failure)
		log.Printf("调用 Simplebank 扣款异常: %v\n", rpcErr)

		// SAGA模式
		// 简单处理：我们假设只要报错就是明确的业务失败（如余额不足），将状态标为 FAILED
		// 进阶处理：如果错误类型是 context.DeadlineExceeded (网络超时)，此时不知道钱到底扣没扣，
		// 应该保留 PAYING 状态，让后台的异步 Worker 去轮询 Simplebank 查账，再决定是改为 PENDING 还是 FAILED。

		updateErr := s.db.WithContext(ctx).Model(bounty).Update("status", models.BountyStatusFailed).Error
		if updateErr != nil {
			log.Printf("严重警告: 状态机回滚失败，bounty_id=%d, err=%v\n", bounty.ID, updateErr)
		}
		return fmt.Errorf("资金冻结失败，悬赏发布终止: %w", rpcErr)
	}

	// gRPC 明确返回成功，更新状态为 PENDING，任务正式进入悬赏大厅
	err = s.db.WithContext(ctx).Model(bounty).Update("status", models.BountyStatusPending).Error
	if err != nil {
		// 极端异常：Simplebank 扣钱成功了，但 Bounty 库自己宕机了没更新成功。
		// 这同样依赖后台 Worker 扫描长时间停留在 PAYING 状态的记录进行最终一致性修复。
		return fmt.Errorf("资金已扣除，但更新本地状态为 PENDING 时出错: %w", err)
	}

	bounty.Status = models.BountyStatusPending
	return nil
}

// AcceptBounty 处理猎人“抢单/申请”逻辑
func (s *SQLStore) AcceptBounty(ctx context.Context, bountyID, hunter_account_id int64, hunterUsername string) (*models.BountyApplication, error) {
	var application models.BountyApplication

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bounty models.Bounty
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&bounty, bountyID).Error; err != nil {
			return err
		}

		if bounty.Status != models.BountyStatusPending {
			return fmt.Errorf("该悬赏当前不可接单，状态为: %s", bounty.Status)
		}

		application = models.BountyApplication{
			BountyID:        bountyID,
			HunterUsername:  hunterUsername,
			HunterAccountID: hunter_account_id,
			Status:          models.AppStatusApplied,
		}
		// 落库
		if err := tx.Create(&application).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &application, nil
}

func (s *SQLStore) ConfirmHunter(ctx context.Context, bountyID int64, applicationID int64, employerUsername string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bounty models.Bounty
		// 加上 FOR UPDATE 锁，防止并发修改
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&bounty, bountyID).Error; err != nil {
			return err
		}

		if bounty.EmployerUsername != employerUsername {
			return fmt.Errorf("权限不足: 只能操作您自己发布的悬赏")
		}
		if bounty.Status != models.BountyStatusPending {
			return fmt.Errorf("悬赏状态不合法，当前状态: %s", bounty.Status)
		}

		// 将选中的申请状态改为 ACCEPTED
		res := tx.Model(&models.BountyApplication{}).
			Where("id = ? AND bounty_id = ?", applicationID, bountyID).
			Update("status", models.AppStatusAccepted)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("找不到对应的申请记录")
		}

		// 将其他落选的申请状态改为 REJECTED
		if err := tx.Model(&models.BountyApplication{}).
			Where("bounty_id = ? AND id != ?", bountyID, applicationID).
			Update("status", models.AppStatusRejected).Error; err != nil {
			return err
		}

		// 将悬赏本身状态改为进行中
		return tx.Model(&bounty).Update("status", models.BountyStatusInProgress).Error
	})
}

func (s *SQLStore) CompleteBounty(ctx context.Context, bountyID int64, employerUsername string, bankClient BankClient, platformAccountID int64) error {
	var rewardAmount int64
	var hunterAccountID int64

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bounty models.Bounty
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&bounty, bountyID).Error; err != nil {
			return err
		}

		if bounty.EmployerUsername != employerUsername {
			return fmt.Errorf("权限不足: 非悬赏发布者")
		}
		if bounty.Status != models.BountyStatusInProgress {
			return fmt.Errorf("悬赏尚未开始或已结束，当前状态: %s", bounty.Status)
		}

		// 找到中标的那个猎人申请记录，提取收款账户
		var app models.BountyApplication
		if err := tx.Where("bounty_id = ? AND status = ?", bountyID, models.AppStatusAccepted).First(&app).Error; err != nil {
			return fmt.Errorf("找不到中标的猎人记录: %w", err)
		}

		rewardAmount = bounty.RewardAmount
		hunterAccountID = app.HunterAccountID

		// 更新状态为 SETTLING 防止重复点击
		return tx.Model(&bounty).Update("status", "SETTLING").Error
	})

	if err != nil {
		return err
	}

	// 发起微服务转账调用
	idempotencyKey := fmt.Errorf("complete_bounty_%d", bountyID).Error()

	// 这里传入 context.Background() 而不是包含雇主 Token 的 ctx
	// 钱是从平台担保账户 -> 猎人账户，必须用平台特权操作！
	rpcErr := bankClient.Transfer(context.Background(), platformAccountID, hunterAccountID, rewardAmount, idempotencyKey)

	if rpcErr != nil {
		// 如果网络超时或银行异常，状态保持为 SETTLING，等待后续对账补偿
		return fmt.Errorf("资金打款到猎人账户失败，需人工介入: %w", rpcErr)
	}

	// 转账成功 标记悬赏完成
	return s.db.WithContext(ctx).Model(&models.Bounty{}).
		Where("id = ?", bountyID).
		Update("status", models.BountyStatusCompleted).Error
}

// Saga 补偿动作
func (s *SQLStore) CancelBounty(ctx context.Context, bountyID int64, employerUsername string, bankClient BankClient, platformAccountID int64) error {
	var refundAmount int64
	var originalEmployerAccountID int64

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bounty models.Bounty
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&bounty, bountyID).Error; err != nil {
			return err
		}

		if bounty.EmployerUsername != employerUsername {
			return fmt.Errorf("权限不足: 只能取消您自己发布的悬赏")
		}

		// 只有在 PENDING (悬赏大厅中，未被任何人接单，或者没选定任何人) 状态下才能取消
		if bounty.Status != models.BountyStatusPending {
			return fmt.Errorf("该悬赏当前状态 (%s) 无法取消，仅支持未开始的任务", bounty.Status)
		}

		refundAmount = bounty.RewardAmount
		originalEmployerAccountID = bounty.EmployerAccountID

		// 将关联的、尚未处理的申请全部变为 REJECTED，保持数据整洁
		tx.Model(&models.BountyApplication{}).
			Where("bounty_id = ? AND status = ?", bountyID, models.AppStatusApplied).
			Update("status", models.AppStatusRejected)

		// 更新悬赏状态为 REFUNDING (退款中) 防止重复点击和并发竞争
		return tx.Model(&bounty).Update("status", "REFUNDING").Error
	})

	if err != nil {
		return err
	}

	idempotencyKey := fmt.Errorf("cancel_bounty_refund_%d", bountyID).Error()
	rpcErr := bankClient.Transfer(context.Background(), platformAccountID, originalEmployerAccountID, refundAmount, idempotencyKey)

	if rpcErr != nil {
		log.Printf("严重异常: 订单 %d 取消成功，但退款到账户 %d 失败: %v\n", bountyID, originalEmployerAccountID, rpcErr)
		return fmt.Errorf("订单已关闭，但退款请求发送失败，请联系客服: %w", rpcErr)
	}

	return s.db.WithContext(ctx).Model(&models.Bounty{}).
		Where("id = ?", bountyID).
		Update("status", models.BountyStatusCanceled).Error
}
