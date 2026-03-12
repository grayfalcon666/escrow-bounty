package main

import (
	"log"

	"github.com/grayfalcon666/escrow-bounty/db"
)

const (
	dbSource = "postgresql://root:secret@localhost:5432/escrow_db?sslmode=disable"
)

func main() {
	// 初始化数据库连接
	db.InitDB(dbSource)

	// 测试连接是否畅通
	sqlDB, err := db.Client.DB()
	if err != nil {
		log.Fatal("获取 DB 实例失败:", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Fatal("Ping 数据库失败:", err)
	}

	log.Println("MVP 担保微服务启动成功！")

	// 后续这里将用来启动 gRPC 服务器和 gRPC-Gateway
}
