package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DbConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

func ConnectToDB(dbConfig DbConfig) (*pgxpool.Pool, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.DBName)

	dbPool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, err
	}
	return dbPool, nil
}
