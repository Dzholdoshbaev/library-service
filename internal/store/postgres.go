package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"library-service/internal/config"
	"library-service/internal/domain"
)

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store { return &Store{db: db} }
func Open(ctx context.Context, c config.Config, log logger.Interface) (*gorm.DB, *sql.DB, error) {
	db, err := gorm.Open(postgres.Open(c.DatabaseURL), &gorm.Config{Logger: log, DisableAutomaticPing: true, NowFunc: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		return nil, nil, fmt.Errorf("configure PostgreSQL: %w", err)
	}
	pool, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("get connection pool: %w", err)
	}
	pool.SetMaxOpenConns(c.MaxOpen)
	pool.SetMaxIdleConns(c.MaxIdle)
	pool.SetConnMaxLifetime(c.ConnLifetime)
	pool.SetConnMaxIdleTime(c.ConnIdleTime)
	deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		pingCtx, pingCancel := context.WithTimeout(deadline, 3*time.Second)
		err = pool.PingContext(pingCtx)
		pingCancel()
		if err == nil {
			break
		}
		select {
		case <-deadline.Done():
			_ = pool.Close()
			return nil, nil, fmt.Errorf("PostgreSQL unavailable: %w", deadline.Err())
		case <-time.After(time.Second):
		}
	}
	return db, pool, nil
}
func translate(err error, kind string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.NotFound(kind)
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			if pg.ConstraintName == "users_email_key" {
				return domain.E("USER_ALREADY_EXISTS", "email is already registered", 409)
			}
			return domain.E("CONFLICT", "resource already exists", 409)
		case "23503":
			return domain.E("CONFLICT", "related resource is missing or still in use", 409)
		case "23514", "22001", "22003":
			return domain.Validation(map[string]string{"request": "value violates a data constraint"})
		}
	}
	return fmt.Errorf("persistence operation: %w", err)
}
