package db

import (
	"context"
	"fmt"

	simplebankpb "github.com/grayfalcon666/escrow-bounty/simplebankpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type GRPCBankClient struct {
	client      simplebankpb.SimpleBankClient
	systemToken string
}

func NewGRPCBankClient(cc grpc.ClientConnInterface, systemToken string) *GRPCBankClient {
	return &GRPCBankClient{
		client:      simplebankpb.NewSimpleBankClient(cc),
		systemToken: systemToken,
	}
}

func (c *GRPCBankClient) Transfer(ctx context.Context, fromAccountID, toAccountID, amount int64, idempotencyKey string) error {
	var outgoingCtx context.Context

	md, ok := metadata.FromIncomingContext(ctx)

	if ok && len(md.Get("authorization")) > 0 {
		// 场景 A：前端用户主动触发 (如：Alice 发布悬赏)。将 Alice 的 Token 透传给下游 Simple Bank！
		outgoingCtx = metadata.NewOutgoingContext(context.Background(), md)
	} else {
		// 场景 B：微服务后台异步触发 (如：平台把钱结算给猎人)。此时没有前端 Token，使用系统 Token。
		systemMD := metadata.Pairs("authorization", "Bearer "+c.systemToken)
		outgoingCtx = metadata.NewOutgoingContext(context.Background(), systemMD)
	}

	req := &simplebankpb.TransferTxRequest{
		FromAccountId:  fromAccountID,
		ToAccountId:    toAccountID,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
	}

	_, err := c.client.Transfer(outgoingCtx, req)
	if err != nil {
		return fmt.Errorf("调用 Simple Bank 真实转账接口失败: %w", err)
	}

	return nil
}
