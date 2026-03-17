package gapi

import (
	"context"
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

func TestConfirmHunterAPI(t *testing.T) {
	employerUsername := "alice"
	bountyID := int64(1)
	applicationID := int64(100)

	req := &pb.ConfirmHunterRequest{
		BountyId:      bountyID,
		ApplicationId: applicationID,
	}

	// 模拟执行完 Confirm 后，数据库里悬赏及其申请的最终状态
	mockBountyDetail := &models.Bounty{
		ID:               bountyID,
		EmployerUsername: employerUsername,
		Status:           models.BountyStatusInProgress,
		Applications: []models.BountyApplication{
			{ID: 100, Status: models.AppStatusAccepted, HunterUsername: "bob"},
			{ID: 101, Status: models.AppStatusRejected, HunterUsername: "charlie"}, // 模拟落选者被拒绝
		},
	}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.ConfirmHunterResponse, err error)
	}{
		{
			name: "OK_确认选定并返回最新状态",
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				accessToken, _ := tokenMaker.CreateToken(employerUsername, time.Minute)
				md := metadata.MD{"authorization": []string{"Bearer " + accessToken}}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 第一步：调用业务逻辑更新状态
				store.EXPECT().
					ConfirmHunter(gomock.Any(), bountyID, applicationID, employerUsername).
					Times(1).
					Return(nil)

				// 第二步：查询最新详情以返回给前端
				store.EXPECT().
					GetBountyByID(gomock.Any(), bountyID).
					Times(1).
					Return(mockBountyDetail, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ConfirmHunterResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, string(models.BountyStatusInProgress), res.Bounty.Status)
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
			tc.buildStubs(store)

			server := newTestServer(t, store, bankClient)
			ctx := tc.setupAuth(t, context.Background(), server.tokenMaker)

			res, err := server.ConfirmHunter(ctx, req)
			tc.checkResponse(t, res, err)
		})
	}
}
