package gapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	mockdb "github.com/grayfalcon666/escrow-bounty/db/mock"
	"github.com/grayfalcon666/escrow-bounty/models"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"github.com/grayfalcon666/escrow-bounty/token"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestCompleteBountyAPI(t *testing.T) {
	employerUsername := "alice"
	bountyID := int64(1)

	req := &pb.CompleteBountyRequest{
		BountyId: bountyID,
	}

	mockCompletedBounty := &models.Bounty{
		ID:               bountyID,
		EmployerUsername: employerUsername,
		Status:           models.BountyStatusCompleted,
	}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context
		buildStubs    func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient)
		checkResponse func(t *testing.T, res *pb.CompleteBountyResponse, err error)
	}{
		{
			name: "OK_验收打款成功",
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				accessToken, _ := tokenMaker.CreateToken(employerUsername, time.Minute)
				md := metadata.MD{"authorization": []string{"Bearer " + accessToken}}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient) {
				// 期望 API 转发请求给 Store
				store.EXPECT().
					CompleteBounty(gomock.Any(), bountyID, employerUsername, gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetBountyByID(gomock.Any(), bountyID).
					Times(1).
					Return(mockCompletedBounty, nil)
			},
			checkResponse: func(t *testing.T, res *pb.CompleteBountyResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, string(models.BountyStatusCompleted), res.Bounty.Status)
			},
		},
		{
			name: "InternalError_转账事务失败",
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				accessToken, _ := tokenMaker.CreateToken(employerUsername, time.Minute)
				md := metadata.MD{"authorization": []string{"Bearer " + accessToken}}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient) {
				// 如果底层扣款失败，Store 会返回 Error
				store.EXPECT().
					CompleteBounty(gomock.Any(), bountyID, employerUsername, gomock.Any(), gomock.Any()).
					Times(1).
					Return(fmt.Errorf("资金打款到猎人账户失败"))

				// 因为上一步直接报错 return，所以绝不会查数据库获取最新状态
				store.EXPECT().GetBountyByID(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.CompleteBountyResponse, err error) {
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

			res, err := server.CompleteBounty(ctx, req)
			tc.checkResponse(t, res, err)
		})
	}
}
