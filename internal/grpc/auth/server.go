package auth

import (
	"context"
	"errors"
	"sso/internal/domain/models"
	"sso/internal/services/auth"
	"sso/internal/storage"

	ssov1 "github.com/wadt3rr/city-events-auth-protos/gen/go/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type serverAPI struct {
	ssov1.UnimplementedAuthServer
	auth Auth
}

type Auth interface {
	Login(ctx context.Context, email string, password string, appID int) (token string, err error)
	RegisterNewUser(ctx context.Context, email string, password string, role string) (userID int64, err error)

	GetUserRole(ctx context.Context, userID int64) (role string, err error)
	UpdateRole(ctx context.Context, userID int64, role string) (err error)
	ListUsers(ctx context.Context) ([]models.User, error)
}

func Register(gRPCServer *grpc.Server, auth Auth) {
	ssov1.RegisterAuthServer(gRPCServer, &serverAPI{auth: auth})
}

func (s *serverAPI) Login(
	ctx context.Context, in *ssov1.LoginRequest,
) (response *ssov1.LoginResponse, err error) {
	if in.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	if in.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	if in.GetAppId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}

	token, err := s.auth.Login(ctx, in.GetEmail(), in.GetPassword(), int(in.GetAppId()))
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, "invalid email or password")
		}
		return nil, status.Error(codes.Internal, "failed to login")
	}

	return &ssov1.LoginResponse{Token: token}, nil
}

func (s *serverAPI) Register(
	ctx context.Context,
	in *ssov1.RegisterRequest,
) (*ssov1.RegisterResponse, error) {
	if in.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if in.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	uid, err := s.auth.RegisterNewUser(ctx, in.GetEmail(), in.GetPassword(), in.GetRole())
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}
		return nil, status.Error(codes.Internal, "failed to register")
	}

	return &ssov1.RegisterResponse{UserId: uid}, nil
}

func (s *serverAPI) GetUserRole(ctx context.Context, in *ssov1.GetUserRoleRequest) (*ssov1.GetUserRoleResponse, error) {
	role, err := s.auth.GetUserRole(ctx, in.GetUserId())
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to get user")
	}
	return &ssov1.GetUserRoleResponse{Role: role}, nil
}

func (s *serverAPI) UpdateRole(ctx context.Context, in *ssov1.UpdateUserRoleRequest) (*ssov1.UpdateUserRoleResponse, error) {
	err := s.auth.UpdateRole(ctx, in.GetUserId(), in.GetRole())
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to update user")
	}
	return &ssov1.UpdateUserRoleResponse{}, nil
}

func (s *serverAPI) ListUsers(ctx context.Context, request *ssov1.ListUsersRequest) (*ssov1.ListUsersResponse, error) {
	users, err := s.auth.ListUsers(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list users")
	}

	resp := &ssov1.ListUsersResponse{}
	for _, user := range users {
		resp.Users = append(resp.Users, &ssov1.User{
			Id:    user.ID,
			Email: user.Email,
			Role:  user.Role,
		})
	}
	return resp, nil
}
