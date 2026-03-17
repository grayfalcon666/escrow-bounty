package gapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	mockdb "github.com/grayfalcon666/escrow-bounty/db/mock"
	"github.com/grayfalcon666/escrow-bounty/models"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"github.com/grayfalcon666/escrow-bounty/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/metadata"
)

func TestCancelBountyAPI(t *testing.T) {
	employerUsername := "alice"
	bountyID := int64(1)

	req := &pb.CancelBountyRequest{
		BountyId: bountyID,
	}

	mockCancelledBounty := &models.Bounty{
		ID:               bountyID,
		EmployerUsername: employerUsername,
		Status:           "CANCELLED",
	}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context
		buildStubs    func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient)
		checkResponse func(t *testing.T, res *pb.CancelBountyResponse, err error)
	}{
		{
			name: "OK_取消退款成功",
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				accessToken, _ := tokenMaker.CreateToken(employerUsername, time.Minute)
				md := metadata.MD{"authorization": []string{"Bearer " + accessToken}}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient) {
				// 期望 API 调用 Store 层的取消补偿逻辑
				store.EXPECT().
					CancelBounty(gomock.Any(), bountyID, employerUsername, gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetBountyByID(gomock.Any(), bountyID).
					Times(1).
					Return(mockCancelledBounty, nil)
			},
			checkResponse: func(t *testing.T, res *pb.CancelBountyResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, "CANCELLED", res.Bounty.Status)
			},
		},
		{
			name: "InternalError_取消退款失败",
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				accessToken, _ := tokenMaker.CreateToken(employerUsername, time.Minute)
				md := metadata.MD{"authorization": []string{"Bearer " + accessToken}}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient) {
				// 模拟退款由于某种原因报错
				store.EXPECT().
					CancelBounty(gomock.Any(), bountyID, employerUsername, gomock.Any(), gomock.Any()).
					Times(1).
					Return(fmt.Errorf("订单已关闭，但退款请求发送失败"))

				// 直接抛错，不再查询最新状态
				store.EXPECT().GetBountyByID(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.CancelBountyResponse, err error) {
				require.Error(t, err)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			bankClient := mockdb.NewMockBankClient(ctrl)
			tc.buildStubs(store, bankClient)

			server := newTestServer(t, store, bankClient)
			ctx := tc.setupAuth(t, context.Background(), server.tokenMaker)

			res, err := server.CancelBounty(ctx, req)
			tc.checkResponse(t, res, err)
		})
	}
}
