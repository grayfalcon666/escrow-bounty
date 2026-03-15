package gapi

import (
	"context"

	"github.com/grayfalcon666/escrow-bounty/models"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (server *Server) CreateBounty(ctx context.Context, req *pb.CreateBountyRequest) (*pb.CreateBountyResponse, error) {
	authPayload, err := server.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	// TODO 替换为validator进行校验
	if req.GetRewardAmount() <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "悬赏金额必须大于 0")
	}

	bounty := &models.Bounty{
		EmployerUsername: authPayload.Username,
		Title:            req.GetTitle(),
		Description:      req.GetDescription(),
		RewardAmount:     req.GetRewardAmount(),
		// Status 不需要在这里赋值 PublishBounty 事务里会将其初始化为 PAYING
	}

	employerAccountID := req.GetEmployerAccountId()
	if employerAccountID <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "必须指定支付账户 ID")
	}

	// 设定平台担保账户的 ID
	platformEscrowAccountID := int64(4)

	// 调用带有 Saga 分布式事务逻辑的 Store 方法
	err = server.store.PublishBounty(ctx, bounty, server.bankClient, employerAccountID, platformEscrowAccountID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "发布悬赏失败: %v", err)
	}

	return &pb.CreateBountyResponse{
		Bounty: convertBounty(bounty),
	}, nil
}
