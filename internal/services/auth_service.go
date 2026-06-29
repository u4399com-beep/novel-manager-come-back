package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
)

const minPasswordLen = 8

// HashPassword generates a bcrypt hash.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword compares password against hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CreateAccessToken generates a JWT.
func CreateAccessToken(cfg *config.Config, userID, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Duration(cfg.AccessTokenExpireMin) * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.SecretKey))
}

// ParseAccessToken validates and returns claims.
func ParseAccessToken(cfg *config.Config, tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.SecretKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// RegisterUser creates a new user account.
func RegisterUser(username, email, password string) (*models.User, error) {
	if len(password) < minPasswordLen {
		return nil, ErrPasswordTooShort
	}

	var count int64
	database.DB.Model(&models.User{}).Where("username = ? OR email = ?", username, email).Count(&count)
	if count > 0 {
		return nil, ErrUserExists
	}

	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Username:       username,
		Email:          email,
		HashedPassword: hashed,
	}
	if err := database.DB.Create(user).Error; err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// AuthenticateUser verifies credentials and returns the user.
func AuthenticateUser(username, password string) (*models.User, error) {
	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrInvalidCredentials
	}
	if !CheckPassword(password, user.HashedPassword) {
		return nil, ErrInvalidCredentials
	}
	return &user, nil
}

// GetUserByID retrieves a user.
func GetUserByID(userID string) (*models.User, error) {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUser applies safe partial updates. Only allowlisted fields are accepted
// to prevent privilege escalation (changing role, is_active, etc.).
func UpdateUser(userID string, updates map[string]interface{}) (*models.User, error) {
	user, err := GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	// Allowlist: only these fields can be self-updated
	safeUpdates := make(map[string]interface{})
	allowedKeys := map[string]bool{
		"username": true,
		"email":    true,
	}

	for k, v := range updates {
		if allowedKeys[k] {
			safeUpdates[k] = v
		}
	}

	// Handle password separately (requires hashing)
	if pw, ok := updates["password"].(string); ok {
		if len(pw) < minPasswordLen {
			return nil, ErrPasswordTooShort
		}
		hashed, err := HashPassword(pw)
		if err != nil {
			return nil, err
		}
		safeUpdates["hashed_password"] = hashed
	}

	if len(safeUpdates) == 0 {
		return user, nil
	}

	if err := database.DB.Model(user).Updates(safeUpdates).Error; err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return GetUserByID(userID)
}
