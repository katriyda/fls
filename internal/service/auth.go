package service

import (
	"bufio"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	db *sql.DB
}

func NewAuth(db *sql.DB) *Auth {
	return &Auth{db: db}
}

func (a *Auth) IsPasswordSet() (bool, error) {
	var count int
	err := a.db.QueryRow("SELECT COUNT(*) FROM config WHERE key = 'admin_password'").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check password: %w", err)
	}
	return count > 0, nil
}

func (a *Auth) SetupPasswordWizard() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n=== FLS First-Time Setup ===")
	fmt.Println("No admin password is configured.")
	fmt.Println("Please set an admin password to secure your file sharing system.")

	for {
		fmt.Print("Enter admin password: ")
		password, _ := reader.ReadString('\n')
		password = strings.TrimSpace(password)

		if len(password) < 6 {
			fmt.Println("Password must be at least 6 characters.")
			continue
		}

		fmt.Print("Confirm password: ")
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(confirm)

		if password != confirm {
			fmt.Println("Passwords do not match. Try again.")
			continue
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}

		_, err = a.db.Exec(
			"INSERT OR REPLACE INTO config (key, value, updated_at) VALUES ('admin_password', ?, CURRENT_TIMESTAMP)",
			string(hash),
		)
		if err != nil {
			return fmt.Errorf("save password: %w", err)
		}

		fmt.Println("Admin password set successfully!")
		slog.Info("admin password configured")
		return nil
	}
}

func (a *Auth) VerifyPassword(password string) (bool, error) {
	var hash string
	err := a.db.QueryRow("SELECT value FROM config WHERE key = 'admin_password'").Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("get password hash: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func GenerateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
