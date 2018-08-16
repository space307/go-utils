package database

import (
	"database/sql"
	"fmt"
	"net"

	"github.com/lib/pq"
)

// BuildPostgresDSN returns Postgres connect string for given config.
func BuildPostgresDSN(config *Config) string {
	const dsn = "postgres://%s:%s@%s/%s?connect_timeout=%d"
	result := fmt.Sprintf(dsn, config.User, config.Password, config.Addr, config.Database, config.TimeoutMs/1000)
	if len(config.SSLMode) > 0 {
		result += "&sslmode=" + config.SSLMode
	}
	return result
}

type pqImpl struct{}

func (i pqImpl) dsn(config *Config) string {
	return BuildPostgresDSN(config)
}

func (i pqImpl) isConnectionError(err error) bool {
	if _, ok := err.(*net.OpError); ok {
		return true
	}
	return pqErrHasClass(err, "08")
}

func (i pqImpl) initTx(tx *sql.Tx) error {
	_, err := tx.Exec("SET transaction ISOLATION LEVEL READ COMMITTED")
	return err
}

func pqErrHasCode(err error, code string) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		if string(pqErr.Code) == code {
			return true
		}
	}
	return false
}

func pqErrHasClass(err error, class string) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		if string(pqErr.Code.Class()) == class {
			return true
		}
	}
	return false
}

func isPqDupErr(err error) bool {
	return pqErrHasCode(err, "23505")
}
