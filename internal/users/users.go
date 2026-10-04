package users

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrUserEmailNotFound      = errors.New("user with the email not found")
	ErrUserEmailAlreadyExists = errors.New("user with the email already exists")
	QueryTimeoutDuration      = 5 * time.Second
)

type User struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Password []byte `json:"-"`
}

// ValidateForCreate runs the validation required for when a user is being created. i.e. ID is not available
func (us *User) ValidateForCreate() error {
	if us.FullName == "" {
		return errors.New("validation: full name cannot be empty")
	}

	if us.Email == "" {
		return errors.New("validation: email cannot be empty")
	}

	if len(us.Password) == 0 {
		return errors.New("validation: password cannot be empty")
	}

	return nil
}

func (us *User) Sanitize() {
	us.ID = strings.TrimSpace(us.ID)
	us.FullName = strings.TrimSpace(us.FullName)
	us.Email = strings.TrimSpace(us.Email)
}

func (us *User) HashPassword() error {
	hashed, err := bcrypt.GenerateFromPassword(us.Password, bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	us.Password = hashed
	return nil
}

func (us *User) CheckPassword(plain string) bool {
	err := bcrypt.CompareHashAndPassword(us.Password, []byte(plain))
	return err == nil
}

type store interface {
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	SaveUser(ctx context.Context, user *User) (string, error)
	BulkSaveUser(ctx context.Context, users []User) error
	UpdatePassword(ctx context.Context, id string, newPassword []byte) error

	SaveRefreshToken(ctx context.Context, jti, userID string, expiresAt time.Time) error
	CheckRefreshToken(ctx context.Context, jti string) (bool, error)
	RevokeRefreshToken(ctx context.Context, jti string) error
}
type Users struct {
	store store
}

func (us *Users) Register(ctx context.Context, user *User) (*User, error) {
	user.Sanitize()
	err := user.ValidateForCreate()
	if err != nil {
		return nil, err
	}

	if err := user.HashPassword(); err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	newID, err := us.store.SaveUser(ctx, user)
	if err != nil {
		return nil, err
	}
	user.ID = newID

	return user, nil
}

func (us *Users) ReadByID(ctx context.Context, id string) (*User, error) {
	if id == "" {
		return nil, errors.New("validation: no id provided")
	}

	return us.store.GetUserByID(ctx, id)
}

func (us *Users) AsyncRegisters(ctx context.Context, users []User) error {
	errList := make([]error, 0, len(users))
	for i := range users {
		err := users[i].ValidateForCreate()
		if err != nil {
			errList = append(errList, err)
			continue
		}

		if err := users[i].HashPassword(); err != nil {
			errList = append(errList, err)
		}
	}

	if len(errList) != 0 {
		return errors.Join(errList...)
	}

	go func() {
		ctx := context.TODO()
		err := us.store.BulkSaveUser(context.TODO(), users)
		if err != nil {
			slog.ErrorContext(ctx, "bulk save user failed", "error", err, "users", users)
		}
	}()

	return nil
}

func (us *Users) Login(ctx context.Context, email, password string) (*User, error) {
	user, err := us.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if !user.CheckPassword(password) {
		return nil, errors.New("invalid credentials")
	}
	return user, nil
}

func (us *Users) ChangePassword(ctx context.Context, id, oldPassword, newPassword string) error {
	user, err := us.store.GetUserByID(ctx, id)
	if err != nil {
		return err
	}

	if !user.CheckPassword(oldPassword) {
		return errors.New("invalid credentials")
	}

	user.Password = []byte(newPassword)
	if err := user.HashPassword(); err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := us.store.UpdatePassword(ctx, id, user.Password); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

func (us *Users) SaveRefreshToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	return us.store.SaveRefreshToken(ctx, jti, userID, expiresAt)
}

func (us *Users) CheckRefreshToken(ctx context.Context, jti string) (bool, error) {
	return us.store.CheckRefreshToken(ctx, jti)
}

func (us *Users) RevokeRefreshToken(ctx context.Context, jti string) error {
	return us.store.RevokeRefreshToken(ctx, jti)
}

func NewService(store store) *Users {
	return &Users{
		store: store,
	}
}
