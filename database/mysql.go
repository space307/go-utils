package database

import (
	"database/sql"
	"fmt"
	"net"
	"regexp"

	"github.com/go-sql-driver/mysql"
)

var (
	mysqlDuplicateErrorRegex = regexp.MustCompile(`Error 1062:`)
	mysqlConErrorRegex       = regexp.MustCompile(`Error (2002|2003):`)
)

// BuildMySQLDSN returns MySQL connect string for given config.
func BuildMySQLDSN(config *Config) string {
	const templ = "%s:%s@tcp(%s)/%s?charset=%s&timeout=%dms&readTimeout=%dms&writeTimeout=%dms&tx_isolation='READ-COMMITTED'"
	if len(config.Charset) == 0 {
		config.Charset = "utf8"
	}
	dsn := fmt.Sprintf(
		templ, config.User, config.Password, config.Addr,
		config.Database, config.Charset, config.TimeoutMs, config.ReadTimeoutMs,
		config.WriteTimeoutMs,
	)
	if config.ParseTime {
		dsn += fmt.Sprintf("&parseTime=%t", config.ParseTime)
	}
	if len(config.Timezone) > 1 {
		dsn += fmt.Sprintf("&loc=%s", config.Timezone)
	}
	return dsn
}

type mysqlImpl struct{}

func (i mysqlImpl) dsn(config *Config) string {
	return BuildMySQLDSN(config)
}

func (i mysqlImpl) isConnectionError(err error) bool {
	if _, ok := err.(*net.OpError); ok {
		return true
	}
	return mysqlConErrorRegex.MatchString(err.Error())
}

func (i mysqlImpl) initTx(tx *sql.Tx) error {
	_, err := tx.Exec("SET tx_isolation = 'READ-COMMITTED'")
	return err
}

func mysqlErrHasCode(err error, code int) bool {
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		if int(mysqlErr.Number) == code {
			return true
		}
	}
	return false
}

func isMySQLDupErr(err error) bool {
	if err != nil {
		return mysqlDuplicateErrorRegex.MatchString(err.Error())
	}
	return false
}
