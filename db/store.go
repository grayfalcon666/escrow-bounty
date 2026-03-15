package db

import (
	"context"
	"fmt"
	"log"

	"github.com/grayfalcon666/escrow-bounty/models"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{
		db: db,
	}
}

// ==========================================
// Bounties (悬赏操作)
// ==========================================
func (s *Store) CreateBounty(ctx context.Context, bounty *models.Bounty) error {
	return s.db.WithContext(ctx).Create(bounty).Error
}

// 根据 ID 获取单个悬赏详情，并使用 Preload 预加载关联的申请列表
func (s *Store) GetBountyByID(ctx context.Context, id int64) (*models.Bounty, error) {
	var bounty models.Bounty
	err := s.db.WithContext(ctx).Preload("Applications").First(&bounty, id).Error
	if err != nil {
		return nil, err
	}
	return &bounty, nil
}

// 分页获取悬赏列表，支持按状态过滤
func (s *Store) ListBounties(ctx context.Context, status models.BountyStatus, limit, offset int) ([]models.Bounty, error) {
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
func (s *Store) UpdateBountyStatus(ctx context.Context, id int64, status models.BountyStatus) error {
	return s.db.WithContext(ctx).
		Model(&models.Bounty{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// ==========================================
// Bounty Applications (接单申请操作)
// ==========================================

func (s *Store) CreateApplication(ctx context.Context, app *models.BountyApplication) error {
	return s.db.WithContext(ctx).Create(app).Error
}

func (s *Store) UpdateApplicationStatus(ctx context.Context, applicationID int64, status models.ApplicationStatus) error {
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

func (s *Store) PublishBounty(ctx context.Context, bounty *models.Bounty, bankClient BankClient, employerAccountID, platformEscrowAccountID int64) error {

	bounty.Status = models.BountyStatusPaying
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
func (s *Store) AcceptBounty(ctx context.Context, bountyID int64, hunterUsername string) (*models.BountyApplication, error) {
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
			BountyID:       bountyID,
			HunterUsername: hunterUsername,
			Status:         models.AppStatusApplied,
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
