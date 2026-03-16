package gapi

import (
	"context"

	"github.com/grayfalcon666/escrow-bounty/pb"
)

func (server *Server) CompleteBounty(ctx context.Context, req *pb.CompleteBountyRequest) (*pb.CompleteBountyResponse, error) {
	authPayload, err := server.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	platformEscrowAccountID := int64(3) //TODO 用viper统一管理配置文件

	err = server.store.CompleteBounty(ctx, req.GetBountyId(), authPayload.Username, server.bankClient, platformEscrowAccountID)
	if err != nil {
		return nil, err
	}

	bounty, _ := server.store.GetBountyByID(ctx, req.GetBountyId())
	return &pb.CompleteBountyResponse{Bounty: convertBounty(bounty)}, nil
}
