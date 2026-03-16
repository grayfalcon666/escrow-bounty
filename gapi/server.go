package gapi

import (
	"github.com/grayfalcon666/escrow-bounty/db"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"github.com/grayfalcon666/escrow-bounty/token"
	"github.com/grayfalcon666/escrow-bounty/util"
)

type Server struct {
	pb.UnimplementedEscrowBountyServiceServer
	config     util.Config
	tokenMaker *token.JWTMaker
	store      db.Store
	bankClient db.BankClient
}

func NewServer(config util.Config, store db.Store, bankClient db.BankClient, tokenMaker *token.JWTMaker) *Server {
	return &Server{
		config:     config,
		store:      store,
		bankClient: bankClient,
		tokenMaker: tokenMaker,
	}
}
