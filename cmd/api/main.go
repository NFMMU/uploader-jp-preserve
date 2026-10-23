package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/uploader-jp/uploader-archive/internal/api"
	"github.com/uploader-jp/uploader-archive/internal/config"
	"github.com/uploader-jp/uploader-archive/internal/gen/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	gormLogLevel := gormlogger.Warn
	if cfg.Environment == "development" {
		gormLogLevel = gormlogger.Info
	}
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{Logger: gormlogger.Default.LogMode(gormLogLevel), TranslateError: true})
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := sqlDB.PingContext(ctx); err != nil {
		cancel()
		log.Fatalf("database ping failed: %v", err)
	}
	cancel()

	if err := bootstrapAdmin(db, cfg); err != nil {
		log.Fatalf("bootstrap administrator: %v", err)
	}

	handler := api.New(db, cfg, logger)
	srv := &http.Server{Addr: cfg.Address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Minute, WriteTimeout: 30 * time.Minute, IdleTimeout: 90 * time.Second}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api_starting", "address", cfg.Address, "environment", cfg.Environment)
		serverErr <- srv.ListenAndServe()
	}()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	case <-stopCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful_shutdown_failed", "error", err)
		}
	}
}

func bootstrapAdmin(db *gorm.DB, cfg config.Config) error {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if cfg.BootstrapAdminEmail == "" || cfg.BootstrapAdminPassword == "" {
		return errors.New("no users exist; set BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD for first startup")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(cfg.BootstrapAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user := model.User{Email: strings.ToLower(strings.TrimSpace(cfg.BootstrapAdminEmail)), PasswordHash: string(passwordHash), DisplayName: "Initial Administrator", Role: "admin"}
	if err := db.Create(&user).Error; err != nil {
		return err
	}
	return db.Create(&model.AuditLog{ActorID: &user.ID, ActorLabel: user.Email, TargetType: "user", TargetID: &user.ID, Action: "user.bootstrap_admin.create", Result: "success", Reason: ptr("Initial administrator created from environment configuration")}).Error
}

func ptr(value string) *string { return &value }
