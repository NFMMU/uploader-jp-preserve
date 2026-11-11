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

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/nfmmu/uploader-archive/internal/api"
	"github.com/nfmmu/uploader-archive/internal/config"
	"github.com/nfmmu/uploader-archive/internal/gen/model"
	"github.com/nfmmu/uploader-archive/internal/gen/query"
)

func main() {
	// ロガーのセットアップ
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// 設定のロード
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	// gormのログレベルを環境に応じて設定
	gormLogLevel := gormlogger.Warn
	if cfg.Environment == "development" {
		gormLogLevel = gormlogger.Info
	}
	// データベース接続の確立
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{Logger: gormlogger.Default.LogMode(gormLogLevel), TranslateError: true})
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	// データベース接続の設定
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

	// queryパッケージの初期化
	query.SetDefault(db)

	// 管理者ユーザーのブートストラップ
	if err := bootstrapAdmin(cfg); err != nil {
		log.Fatalf("bootstrap administrator: %v", err)
	}

	// APIサーバーの起動
	handler := api.New(db, cfg, logger)
	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Minute,
		WriteTimeout:      30 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api_starting", "address", cfg.Address, "environment", cfg.Environment)
		serverErr <- srv.ListenAndServe()
	}()

	// シャットダウンのシグナルを待機
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

func bootstrapAdmin(cfg config.Config) error {
	count, err := query.User.WithContext(context.Background()).Count()
	if err != nil {
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
	if err := query.User.WithContext(context.Background()).Create(&user); err != nil {
		return err
	}
	return query.AuditLog.WithContext(context.Background()).Create(
		&model.AuditLog{
			ActorID:    &user.ID,
			ActorLabel: user.Email,
			TargetType: "user",
			TargetID:   &user.ID,
			Action:     "user.bootstrap_admin.create",
			Result:     "success",
			Reason:     new("Initial administrator created from environment configuration"),
		})
}
