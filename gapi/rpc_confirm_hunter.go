package gapi

import (
	"context"

	"github.com/grayfalcon666/escrow-bounty/pb"
)

func (server *Server) ConfirmHunter(ctx context.Context, req *pb.ConfirmHunterRequest) (*pb.ConfirmHunterResponse, error) {
	authPayload, err := server.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	err = server.store.ConfirmHunter(ctx, req.GetBountyId(), req.GetApplicationId(), authPayload.Username)
	if err != nil {
		return nil, err // TODO 实际生产需转换 status.Errorf
	}

	// 返回最新状态
	bounty, _ := server.store.GetBountyByID(ctx, req.GetBountyId())
	return &pb.ConfirmHunterResponse{Bounty: convertBounty(bounty)}, nil
}
