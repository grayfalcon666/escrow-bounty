package gapi

import (
	"github.com/grayfalcon666/escrow-bounty/models"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func convertBounty(bounty *models.Bounty) *pb.Bounty {
	return &pb.Bounty{
		Id:          bounty.ID,
		EmployerId:  bounty.EmployerID,
		Title:       bounty.Title,
		Description: bounty.Description,
		Status:      string(bounty.Status),
		CreatedAt:   timestamppb.New(bounty.CreatedAt),
		UpdatedAt:   timestamppb.New(bounty.UpdatedAt),
	}
}

func convertBountyApplication(app *models.BountyApplication) *pb.BountyApplication {
	return &pb.BountyApplication{
		Id:        app.ID,
		BountyId:  app.BountyID,
		HunterId:  app.HunterID,
		Status:    string(app.Status),
		CreatedAt: timestamppb.New(app.CreatedAt),
		UpdatedAt: timestamppb.New(app.UpdatedAt),
	}
}
