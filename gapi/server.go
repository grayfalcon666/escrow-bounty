package gapi

import (
	"github.com/grayfalcon666/escrow-bounty/db"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"github.com/grayfalcon666/escrow-bounty/token"
)

type Server struct {
	pb.UnimplementedEscrowBountyServiceServer
	tokenMaker *token.JWTMaker
	store      *db.Store
	bankClient db.BankClient
}

func NewServer(store *db.Store, bankClient db.BankClient, tokenMaker *token.JWTMaker) *Server {
	return &Server{
		store:      store,
		bankClient: bankClient,
		tokenMaker: tokenMaker,
	}
}
