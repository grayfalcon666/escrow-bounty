package main

import (
	"context"
	"log"
	"net"
	"net/http"

	"github.com/grayfalcon666/escrow-bounty/db"
	"github.com/grayfalcon666/escrow-bounty/gapi"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	dbSource          = "postgresql://root:secret@localhost:5432/escrow_db?sslmode=disable"
	grpcServerAddress = "0.0.0.0:9097"
	httpServerAddress = "0.0.0.0:8087"
)

func main() {
	db.InitDB(dbSource)
	store := db.NewStore(db.Client)
	bankClient := &db.MockBankClient{}

	server := gapi.NewServer(store, bankClient)

	go runGatewayServer(server)
	runGrpcServer(server)
}

func runGrpcServer(server pb.EscrowBountyServiceServer) {
	grpcServer := grpc.NewServer()
	pb.RegisterEscrowBountyServiceServer(grpcServer, server)
	reflection.Register(grpcServer) // 开启反射

	listener, err := net.Listen("tcp", grpcServerAddress)
	if err != nil {
		log.Fatalf("无法监听 gRPC 端口: %v", err)
	}

	log.Printf("启动 gRPC 服务，监听地址: %s", listener.Addr().String())
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("gRPC 服务运行失败: %v", err)
	}
}

func runGatewayServer(server pb.EscrowBountyServiceServer) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()

	err := pb.RegisterEscrowBountyServiceHandlerServer(ctx, grpcMux, server)
	if err != nil {
		log.Fatalf("无法注册 Gateway 处理器: %v", err)
	}

	// 创建标准 HTTP Mux 并将所有请求路由给 grpcMux 处理
	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	// 配置 CORS 中间件
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
		ExposedHeaders:   []string{"Grpc-Metadata-Authorization"},
		AllowCredentials: true,
	})

	handler := corsHandler.Handler(mux)

	listener, err := net.Listen("tcp", httpServerAddress)
	if err != nil {
		log.Fatalf("无法监听 HTTP 端口: %v", err)
	}

	log.Printf("启动 HTTP Gateway 服务，监听地址: %s", listener.Addr().String())
	err = http.Serve(listener, handler)
	if err != nil {
		log.Fatalf("HTTP Gateway 服务运行失败: %v", err)
	}
}
