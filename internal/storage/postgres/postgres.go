package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sso/internal/domain/models"
	"sso/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	pool *pgxpool.Pool
}

func New() (*Storage, error) {
	const op = "storage.postgres.New"

	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		return nil, fmt.Errorf("%s: DATABASE_URL isn't set", op)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: cannot connect to db: %w", op, err)
	}
	return &Storage{pool: pool}, nil
}

func (s *Storage) Close() {
	s.pool.Close()
}

func (s *Storage) SaveUser(
	ctx context.Context,
	email string,
	passHash []byte,
	role string,
) (int64, error) {
	const op = "storage.postgres.SaveUser"

	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users(email, pass_hash, role) 
			VALUES ($1, $2, $3) 
			RETURNING id`,
		email, passHash, role,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrUserExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) User(ctx context.Context, email string) (models.User, error) {
	const op = "storage.postgres.User"

	var user models.User

	err := s.pool.QueryRow(ctx,
		`SELECT id, email, pass_hash, role 
			FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PassHash, &user.Role)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}

		return user, fmt.Errorf("%s: %w", op, err)
	}

	return user, nil
}

func (s *Storage) UserByID(ctx context.Context, userID int64) (models.User, error) {
	const op = "storage.postgres.User"

	var user models.User

	err := s.pool.QueryRow(ctx,
		`SELECT id, email, pass_hash, role 
			FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.PassHash, &user.Role)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}

		return user, fmt.Errorf("%s: %w", op, err)
	}

	return user, nil
}

func (s *Storage) UpdateRole(ctx context.Context, userID int64, role string) error {
	const op = "storage.postgres.UpdateUserRole"

	validRoles := map[string]bool{
		"admin":     true,
		"user":      true,
		"organizer": true,
	}
	if !validRoles[role] {
		return fmt.Errorf("%s: invalid role: %s", op, role)
	}

	res, err := s.pool.Exec(ctx,
		`UPDATE users SET role = $1 WHERE id = $2`, role, userID,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if res.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
	}

	return nil
}

func (s *Storage) App(ctx context.Context, appID int) (models.App, error) {
	const op = "storage.postgres.App"

	var app models.App

	err := s.pool.QueryRow(ctx, `SELECT id, name, secret FROM apps WHERE id = $1`, appID).Scan(&app.ID, &app.Name, &app.Secret)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.App{}, fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}

		return app, fmt.Errorf("%s: %w", op, err)
	}

	return app, nil

}

func (s *Storage) GetUserRole(ctx context.Context, userID int64) (string, error) {
	const op = "storage.postgres.GetUserRole"
	var role string

	err := s.pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}

		return "", fmt.Errorf("%s: %w", op, err)
	}

	return role, nil
}

func (s *Storage) ListUsers(ctx context.Context) ([]models.User, error) {
	const op = "storage.postgres.ListUsers"

	rows, err := s.pool.Query(ctx, `SELECT id, email, pass_hash, role FROM users`)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		err = rows.Scan(&u.ID, &u.Email, &u.PassHash, &u.Role)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return users, nil

}
