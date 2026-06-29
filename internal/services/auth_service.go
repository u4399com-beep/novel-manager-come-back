package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func CreateAccessToken(cfg *config.Config, userID, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID, "role": role, "iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Duration(cfg.AccessTokenExpireMin) * time.Minute).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.SecretKey))
}

func ParseAccessToken(cfg *config.Config, tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.SecretKey), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func RegisterUser(ctx context.Context, username, email, password string) (*models.User, error) {
	if len(password) < 8 {
		return nil, ErrPasswordTooShort
	}
	hashed, _ := HashPassword(password)
	u := &models.User{}
	err := database.Pool.QueryRow(ctx,
		"INSERT INTO users (username, email, hashed_password) VALUES ($1,$2,$3) ON CONFLICT (username) DO NOTHING RETURNING id, username, email, role, is_active, created_at, updated_at",
		username, email, hashed,
	).Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, ErrUserExists
		}
		return nil, err
	}
	return u, nil
}

func AuthenticateUser(ctx context.Context, username, password string) (*models.User, error) {
	u := &models.User{}
	err := database.Pool.QueryRow(ctx,
		"SELECT id, username, email, hashed_password, role, is_active, created_at, updated_at FROM users WHERE username = $1", username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.HashedPassword, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !CheckPassword(password, u.HashedPassword) {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

func GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	u := &models.User{}
	err := database.Pool.QueryRow(ctx,
		"SELECT id, username, email, hashed_password, role, is_active, created_at, updated_at FROM users WHERE id = $1", userID,
	).Scan(&u.ID, &u.Username, &u.Email, &u.HashedPassword, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

func UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*models.User, error) {
	pool := database.Pool
	allowed := map[string]bool{"username": true, "email": true}
	setClauses := []string{}
	args := []interface{}{}
	n := 1
	for k, v := range updates {
		if allowed[k] {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, n))
			args = append(args, v)
			n++
		}
	}
	if pw, ok := updates["password"].(string); ok {
		if len(pw) < 8 {
			return nil, ErrPasswordTooShort
		}
		hashed, _ := HashPassword(pw)
		setClauses = append(setClauses, fmt.Sprintf("hashed_password = $%d", n))
		args = append(args, hashed)
		n++
	}
	if len(setClauses) > 0 {
		sql := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", strings.Join(setClauses, ", "), n)
		args = append(args, userID)
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			return nil, err
		}
	}
	return GetUserByID(ctx, userID)
}
