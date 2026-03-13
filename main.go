package main

import (
	"log"
	"net"

	"github.com/grayfalcon666/escrow-bounty/db"
	"github.com/grayfalcon666/escrow-bounty/gapi"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	dbSource          = "postgresql://root:secret@localhost:5432/escrow_db?sslmode=disable"
	grpcServerAddress = "0.0.0.0:9097"
)

func main() {
	db.InitDB(dbSource)
	store := db.NewStore(db.Client)

	// TODO: 初始化 Simplebank 的 gRPC Client
	var bankClient db.BankClient = nil
	server := gapi.NewServer(store, bankClient)

	grpcServer := grpc.NewServer()
	pb.RegisterEscrowBountyServiceServer(grpcServer, server)

	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", grpcServerAddress)
	if err != nil {
		log.Fatalf("无法监听端口: %v", err)
	}
	log.Printf("启动 gRPC 服务，监听地址: %s", listener.Addr().String())

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("gRPC 服务运行失败: %v", err)
	}
}
