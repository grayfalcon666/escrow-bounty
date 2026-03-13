package gapi

import (
	"github.com/grayfalcon666/escrow-bounty/db"
	"github.com/grayfalcon666/escrow-bounty/pb"
)

type Server struct {
	pb.UnimplementedEscrowBountyServiceServer
	store      *db.Store
	bankClient db.BankClient
}

func NewServer(store *db.Store, bankClient db.BankClient) *Server {
	return &Server{
		store:      store,
		bankClient: bankClient,
	}
}
