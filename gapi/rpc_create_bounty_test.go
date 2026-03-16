package gapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	mockdb "github.com/grayfalcon666/escrow-bounty/db/mock"
	"github.com/grayfalcon666/escrow-bounty/pb"
	"github.com/grayfalcon666/escrow-bounty/token"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestCreateBountyAPI(t *testing.T) {
	employerUsername := "alice"
	employerAccountID := int64(10)
	platformEscrowAccountID := int64(999)
	rewardAmount := int64(5000)

	// 准备前端请求
	req := &pb.CreateBountyRequest{
		Title:             "修复 Bug",
		Description:       "紧急",
		RewardAmount:      rewardAmount,
		EmployerAccountId: employerAccountID,
	}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request context.Context, tokenMaker *token.JWTMaker) context.Context
		buildStubs    func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient)
		checkResponse func(t *testing.T, res *pb.CreateBountyResponse, err error)
	}{
		{
			name: "OK_发布成功",
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				// 模拟签发 JWT Token
				accessToken, err := tokenMaker.CreateToken(employerUsername, time.Minute)
				require.NoError(t, err)
				// 将 Token 塞入 gRPC metadata
				md := metadata.MD{
					"authorization": []string{"Bearer " + accessToken},
				}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient) {
				// 期望业务逻辑调用 PublishBounty，并且没有任何报错
				store.EXPECT().
					PublishBounty(gomock.Any(), gomock.Any(), gomock.Any(), employerAccountID, platformEscrowAccountID).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.CreateBountyResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
			},
		},
		{
			name: "InternalError_SimpleBank扣款失败",
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				accessToken, _ := tokenMaker.CreateToken(employerUsername, time.Minute)
				md := metadata.MD{"authorization": []string{"Bearer " + accessToken}}
				return metadata.NewIncomingContext(requestCtx, md)
			},
			buildStubs: func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient) {
				// 期望业务逻辑调用 PublishBounty，但模拟它返回一个错误（代表转账失败或事务回滚）
				store.EXPECT().
					PublishBounty(gomock.Any(), gomock.Any(), gomock.Any(), employerAccountID, platformEscrowAccountID).
					Times(1).
					Return(fmt.Errorf("资金冻结失败"))
			},
			checkResponse: func(t *testing.T, res *pb.CreateBountyResponse, err error) {
				// 预期收到 gRPC 错误
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Internal, st.Code())
			},
		},
		{
			name: "Unauthenticated_未登录",
			setupAuth: func(t *testing.T, requestCtx context.Context, tokenMaker *token.JWTMaker) context.Context {
				// 不放入任何 Authorization 头
				return requestCtx
			},
			buildStubs: func(store *mockdb.MockStore, bankClient *mockdb.MockBankClient) {
				// 因为鉴权阶段就会被拦截，所以数据库和银行客户端的方法绝对不会被调用 (Times: 0)
				store.EXPECT().PublishBounty(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.CreateBountyResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Unauthenticated, st.Code())
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			// 初始化 gomock 控制器
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 生成 Mock 实例
			store := mockdb.NewMockStore(ctrl)
			bankClient := mockdb.NewMockBankClient(ctrl)

			// 挂载当前用例的桩数据 (Stubs)
			tc.buildStubs(store, bankClient)

			// 启动带有 Mock 的测试服务器
			server := newTestServer(t, store, bankClient)

			// 设置上下文（包含鉴权 Token）
			ctx := tc.setupAuth(t, context.Background(), server.tokenMaker)

			// 触发被测方法
			res, err := server.CreateBounty(ctx, req)

			// 验证结果
			tc.checkResponse(t, res, err)
		})
	}
}
