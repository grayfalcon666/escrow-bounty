package gapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	mockdb "github.com/grayfalcon666/escrow-bounty/db/mock"
	"github.com/grayfalcon666/escrow-bounty/models"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"github.com/grayfalcon666/escrow-bounty/token"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAcceptBountyAPI(t *testing.T) {
	hunterUsername := "bob"
	bountyID := int64(1)
	hunterAccountID := int64(20)

	req := &pb.AcceptBountyRequest{
		BountyId:        bountyID,
		HunterAccountId: hunterAccountID,
	}

	mockApplication := &models.BountyApplication{
		ID:              100,
		BountyID:        bountyID,
		HunterUsername:  hunterUsername,
		HunterAccountID: hunterAccountID,
		Status:          models.AppStatusApplied,
	}

	testCases := []struct {
		name          string
		req           *pb.AcceptBountyRequest
		setupAuth     func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.AcceptBountyResponse, err error)
	}{
		{
			name: "OK_接单成功",
			req:  req,
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				accessToken, _ := tokenMaker.CreateToken(hunterUsername, time.Minute)
				md := metadata.MD{"authorization": []string{"Bearer " + accessToken}}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					AcceptBounty(gomock.Any(), bountyID, hunterAccountID, hunterUsername).
					Times(1).
					Return(mockApplication, nil)
			},
			checkResponse: func(t *testing.T, res *pb.AcceptBountyResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, mockApplication.ID, res.Application.Id)
			},
		},
		{
			name: "AlreadyExists_重复接单",
			req:  req,
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				accessToken, _ := tokenMaker.CreateToken(hunterUsername, time.Minute)
				md := metadata.MD{"authorization": []string{"Bearer " + accessToken}}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 模拟数据库抛出唯一索引冲突错误
				store.EXPECT().
					AcceptBounty(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("duplicate key value violates unique constraint"))
			},
			checkResponse: func(t *testing.T, res *pb.AcceptBountyResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.AlreadyExists, st.Code())
			},
		},
		{
			name: "InvalidArgument_参数错误",
			req: &pb.AcceptBountyRequest{
				BountyId: 0, // 非法 ID
			},
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				accessToken, _ := tokenMaker.CreateToken(hunterUsername, time.Minute)
				md := metadata.MD{"authorization": []string{"Bearer " + accessToken}}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 拦截在参数校验层，不会调用数据库
				store.EXPECT().AcceptBounty(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.AcceptBountyResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.InvalidArgument, st.Code())
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

			res, err := server.AcceptBounty(ctx, tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
