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
	md := metadata.Pairs("authorization", "bearer "+c.systemToken)

	// 创建一个新的外发上下文  把 Metadata 塞进去
	outgoingCtx := metadata.NewOutgoingContext(ctx, md)

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
