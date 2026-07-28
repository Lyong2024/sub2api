package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"golang.org/x/crypto/bcrypt"
)

// EnsureDIYAdminUser creates a bootstrap admin when DIY/SQLite has zero users.
//
// This heals the common Windows packaging footgun: shipping a sample config.yaml
// makes setup.NeedsSetup() false, so AutoSetup never runs and the users table stays empty.
// On every startup we check and create ADMIN_EMAIL / ADMIN_PASSWORD from the environment
// (or defaults) when no users exist.
func EnsureDIYAdminUser(ctx context.Context, client *ent.Client, cfg *config.Config) error {
	if client == nil || cfg == nil {
		return nil
	}
	if !cfg.IsDIY() && !cfg.Database.IsSQLite() {
		return nil
	}

	total, err := client.User.Query().Count(ctx)
	if err != nil {
		return fmt.Errorf("count users for diy bootstrap: %w", err)
	}
	if total > 0 {
		return nil
	}

	email := strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))
	if email == "" {
		email = "admin@sub2api.local"
	}
	password := os.Getenv("ADMIN_PASSWORD")
	if strings.TrimSpace(password) == "" {
		generated, genErr := randomHex(16)
		if genErr != nil {
			return fmt.Errorf("generate admin password: %w", genErr)
		}
		password = generated
		log.Printf("DIY bootstrap: generated one-time admin password for %s: %s", email, password)
		log.Printf("DIY bootstrap: IMPORTANT — save this password; it will not be shown again")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	concurrency := 5
	if strings.EqualFold(strings.TrimSpace(cfg.RunMode), config.RunModeSimple) {
		concurrency = 30
	}

	now := time.Now()
	_, err = client.User.Create().
		SetEmail(email).
		SetPasswordHash(string(hash)).
		SetRole(service.RoleAdmin).
		SetBalance(0).
		SetConcurrency(concurrency).
		SetStatus(service.StatusActive).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create diy admin user: %w", err)
	}

	log.Printf("DIY bootstrap: admin user created email=%s db=%s", email, cfg.Database.SQLitePath())
	return nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
