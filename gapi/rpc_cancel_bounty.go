package gapi

import (
	"context"

	"github.com/grayfalcon666/escrow-bounty/pb"
)

func (server *Server) CancelBounty(ctx context.Context, req *pb.CancelBountyRequest) (*pb.CancelBountyResponse, error) {
	authPayload, err := server.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	// 从配置中拿平台账户 ID
	platformEscrowAccountID := server.config.PlatformEscrowAccountID

	// 调用 Store 层的补偿逻辑
	err = server.store.CancelBounty(ctx, req.GetBountyId(), authPayload.Username, server.bankClient, platformEscrowAccountID)
	if err != nil {
		return nil, err // 可封装为 status.Errorf
	}

	bounty, _ := server.store.GetBountyByID(ctx, req.GetBountyId())

	return &pb.CancelBountyResponse{
		Bounty: convertBounty(bounty),
	}, nil
}
