package database

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMySQLDB(t *testing.T) {
	cfg := &Config{
		Addr:     "127.0.0.1:3306",
		User:     "travis",
		Database: "db_test",
	}
	db, err := InitDatabase(cfg)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS db_test.`t1`(`id` bigint(20) AUTO_INCREMENT,`v` smallint NOT NULL,PRIMARY KEY (`id`),UNIQUE KEY `v` (`v`)) CHARSET=utf8;")
	require.NoError(t, err)

	_, err = db.Exec("delete from `db_test.t1`")
	require.NoError(t, err)

	tx, err := db.StartTransaction()
	require.NoError(t, err)
	_, err = tx.Tx.Exec("insert into t1(v) values(1)")
	require.NoError(t, err)

	err = tx.Tx.Commit()
	require.NoError(t, err)

	_, err = tx.Tx.Exec("insert into t1(v) values(1)")
	require.Error(t, err)
	require.True(t, IsErrorDuplicateKey(err))
}
