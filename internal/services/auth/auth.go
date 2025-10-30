package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sso/internal/domain/models"
	"sso/internal/lib/jwt"
	"sso/internal/lib/logger/sl"
	"sso/internal/storage"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRole        = errors.New("invalid role")
)

type UserSaver interface {
	SaveUser(
		ctx context.Context,
		email string,
		passHash []byte,
		role string,
	) (uid int64, err error)
	UpdateRole(
		ctx context.Context,
		uid int64,
		role string,
	) (err error)
}

type UserProvider interface {
	User(ctx context.Context, email string) (models.User, error)
	UserByID(ctx context.Context, uid int64) (models.User, error)
	ListUsers(ctx context.Context) ([]models.User, error)
	GetUserRole(ctx context.Context, userID int64) (string, error)
}

type AppProvider interface {
	App(ctx context.Context, appID int) (models.App, error)
}

type RoleManager interface {
	UpdateRole(ctx context.Context, userID int64, role string) error
}

type Auth struct {
	log         *slog.Logger
	usrSaver    UserSaver
	usrProvider UserProvider
	appProvider AppProvider
	roleMgr     RoleManager
	tokenTTL    time.Duration
}

func New(log *slog.Logger, userSaver UserSaver, userProvider UserProvider, appProvider AppProvider, roleMgr RoleManager, tokenTTL time.Duration) *Auth {
	return &Auth{
		log:         log,
		usrSaver:    userSaver,
		usrProvider: userProvider,
		appProvider: appProvider,
		roleMgr:     roleMgr,
		tokenTTL:    tokenTTL,
	}
}

func (a *Auth) RegisterNewUser(ctx context.Context, email string, pass string, role string) (int64, error) {
	const op = "Auth.RegisterNewUser"

	log := a.log.With(slog.String("op", op))
	log.Info("registering new user")

	passHash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to hash password", sl.Err(err))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	if role == "" {
		role = "user"
	} else {
		validRoles := map[string]bool{
			"user":      true,
			"organizer": true,
			"admin":     false,
		}
		if !validRoles[role] {
			log.Error("invalid role")

			return 0, fmt.Errorf("%s: %w", op, ErrInvalidRole)
		}
	}

	id, err := a.usrSaver.SaveUser(ctx, email, passHash, role)
	if err != nil {
		log.Error("failed to save user", sl.Err(err))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// Login checks if user with given credentials exists in the system and returns access token.
//
// If user exists, but password is incorrect, returns error.
// If user doesn't exist, returns error.
func (a *Auth) Login(
	ctx context.Context,
	email string,
	password string,
	appID int,
) (string, error) {
	const op = "Auth.Login"

	log := a.log.With(
		slog.String("op", op),
		slog.String("username", email),
		// pass не логируем
	)

	log.Info("attempting to login user")

	// Достаём пользователя из БД
	user, err := a.usrProvider.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			a.log.Warn("user not found", sl.Err(err))

			return "", fmt.Errorf("%s: %w", op, ErrUserNotFound)
		}

		a.log.Error("failed to get user", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	// Проверяем корректность полученного пароля
	if err := bcrypt.CompareHashAndPassword(user.PassHash, []byte(password)); err != nil {
		a.log.Info("invalid credentials", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	// Получаем информацию о приложении
	app, err := a.appProvider.App(ctx, appID)
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged in successfully")

	// Создаём токен авторизации
	token, err := jwt.NewToken(user, app, a.tokenTTL)
	if err != nil {
		a.log.Error("failed to generate token", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	return token, nil
}

func (a *Auth) UpdateRole(ctx context.Context, userID int64, role string) error {
	const op = "Auth.AssignRole"

	log := a.log.With(slog.String("op", op), slog.String("role", role))
	log.Info("attempting to assign role")

	if role != "user" && role != "organizer" && role != "admin" {
		return fmt.Errorf("%s: invalid role: %q", op, role)
	}

	err := a.usrSaver.UpdateRole(ctx, userID, role)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			a.log.Warn("user not found", sl.Err(err))

			return fmt.Errorf("%s: %w", op, ErrUserNotFound)

		}
		return fmt.Errorf("%s: %w", op, err)
	}

	a.log.Info("updated role")
	return nil
}

func (a *Auth) GetUserRole(ctx context.Context, userID int64) (string, error) {
	const op = "Auth.GetRole"

	log := a.log.With(slog.String("op", op), slog.Int64("uid", userID))
	log.Info("attempting to get role")

	role, err := a.usrProvider.GetUserRole(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			a.log.Warn("user not found", sl.Err(err))
			return "", fmt.Errorf("%s: %w", op, ErrUserNotFound)
		}
		a.log.Error("failed to get user role", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	log.Info("role retrieved successfully")

	return role, nil
}

func (a *Auth) ListUsers(ctx context.Context) ([]models.User, error) {
	const op = "Auth.ListUsers"
	log := a.log.With(slog.String("op", op))
	log.Info("attempting to list users")

	users, err := a.usrProvider.ListUsers(ctx)
	if err != nil {
		log.Error("failed to list users", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("users listed successfully")
	return users, nil
}
