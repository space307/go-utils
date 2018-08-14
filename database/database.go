package database

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// cReconnectBanTime time in seconds who means interval between reconnect attempts
	cReconnectBanTime int64 = 5
	// cMaxRetryCount means how will attempts to reconnect
	cMaxRetryCount int = 5
	// cDatabaseStateNotReady means what database not ready to do some work
	cDatabaseStateNotReady int32 = 0
	// cDatabaseStateReady means what database is ready to work
	cDatabaseStateReady int32 = 1
	// cDatabaseStateReconnect means what database now in reconnect process
	cDatabaseStateReconnect int32 = 2

	cHeartBeatWatchInterval         = 60
	cHeartBeatWatchPercent  float64 = 30
)

var (
	// ErrReconBan this error happen when try to reconnect less then cReconnectBanTime interval
	ErrReconBan = errors.New("sql: reconnect banned")
	// ErrNotInitialized this error happen when try to do some work on not initialized database
	ErrNotInitialized = errors.New("sql: database connection not initialized")
	// ErrReconInProcess this error happen when other process do reconnect now
	ErrReconInProcess = errors.New("sql: other reconnect in process")
)

// Executor is an object capable of execution of SQL statements
type Executor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// Config is a struct representing the data needed to connect to a database server
type Config struct {
	Addr           string `yaml:"addr"`
	Database       string `yaml:"database"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	TimeoutMs      int    `yaml:"timeoutms"`
	ReadTimeoutMs  int    `yaml:"readtimeoutms"`
	WriteTimeoutMs int    `yaml:"writetimeoutms"`
	ParseTime      bool   `yaml:"parsetime"`
	Timezone       string `yaml:"timezone"`
	MaxIdleConn    int    `yaml:"maxidleconn"` // maximum idle connections in the pool
	MaxOpenConn    int    `yaml:"maxopenconn"` // maximum pool size
	MaxConnTTL     int    `yaml:"maxconnttl"`  // maximum amount of time a connection may be reused. Calculating since moment connection is opened
	Charset        string `yaml:"charset"`
	SSLMode        string `yaml:"ssl_mode"`
}

// Database is a object extended standard sql.Db structure
// for override methods with implement reconnect logic
type Database struct {
	db        *sql.DB
	config    *Config
	i         impl
	dsn       string
	driver    string
	lastRecon int64
	access    *sync.RWMutex
	isReady   int32
	errors    *errorsWatch
	WarnChan  chan []error
}

// TxConnection is wrapper for transaction with connection link for reset autocommit state after finish transaction
type TxConnection struct {
	Tx  *sql.Tx
	Con *sql.DB
}

// errorsWatch internal structure for collect and analyze data base errors
type errorsWatch struct {
	Step   int64
	Low    int64
	High   int64
	Errors []error
}

type impl interface {
	dsn(config *Config) string
	isConnectionError(err error) bool
	initTx(tx *sql.Tx) error
}

func initDriver(driver string) (impl, error) {
	switch driver {
	case "mysql":
		return &mysqlImpl{}, nil
	case "postgres":
		return &pqImpl{}, nil
	default:
		return nil, errors.New("unknown driver")
	}
}

// InitDatabase creates a mysql storage object based on a given config.
// for compatibility purposes.
func InitDatabase(config *Config) (*Database, error) {
	return Init("mysql", config)
}

// Init creates a storage object based on a given config.
func Init(driver string, config *Config) (*Database, error) {
	impl, err := initDriver(driver)
	if err != nil {
		return nil, err
	}
	extDb := &Database{
		driver:   driver,
		i:        impl,
		dsn:      impl.dsn(config),
		access:   &sync.RWMutex{},
		WarnChan: make(chan []error),
		config:   config,
	}
	if err = extDb.Reconnect(); err != nil {
		return nil, err
	}
	return extDb, nil
}

// GetConfig get database config
func (extDb *Database) GetConfig() *Config {
	return extDb.config
}

// Reconnect safely function which implements loop connection logic
func (extDb *Database) Reconnect() error {
	if atomic.LoadInt32(&extDb.isReady) == cDatabaseStateReconnect {
		return ErrReconInProcess
	}
	extDb.access.Lock()
	atomic.StoreInt32(&extDb.isReady, cDatabaseStateReconnect)
	defer extDb.access.Unlock()

	if time.Now().Unix()-extDb.lastRecon < cReconnectBanTime {
		atomic.StoreInt32(&extDb.isReady, cDatabaseStateNotReady)
		return ErrReconBan
	}

	var try int
	for {
		var err error
		tmpDb, err := sql.Open(extDb.driver, extDb.dsn)
		if err == nil {
			err = tmpDb.Ping()
		}

		if err == nil {

			if extDb.config.MaxIdleConn > 0 {
				tmpDb.SetMaxIdleConns(extDb.config.MaxIdleConn)
			}
			if extDb.config.MaxOpenConn > 0 {
				tmpDb.SetMaxOpenConns(extDb.config.MaxOpenConn)
			}
			if extDb.config.MaxConnTTL > 0 {
				tmpDb.SetConnMaxLifetime(time.Duration(extDb.config.MaxConnTTL) * time.Second)
			}

			if oldDB := extDb.db; oldDB != nil {
				go func() {
					<-time.After(time.Second * 20)
					oldDB.Close()
				}()
			}

			extDb.lastRecon = 0
			extDb.db = tmpDb

			atomic.StoreInt32(&extDb.isReady, cDatabaseStateReady)
			return nil
		}
		if tmpDb != nil {
			tmpDb.Close()
		}
		if try >= cMaxRetryCount {
			extDb.lastRecon = time.Now().Unix()
			atomic.StoreInt32(&extDb.isReady, cDatabaseStateNotReady)
			// store error
			extDb.watchErrors(err)
			return err
		}
		try++
	}
}

// watchErrors collect and analyze data base errors
// send slice of errors if reach the limits
func (extDb *Database) watchErrors(err error) {
	log.Warn("Database connection error: ", err)
	t := time.Now().Unix()
	thisStep := t / cHeartBeatWatchInterval
	// if need to create new one
	if extDb.errors == nil || extDb.errors.Step != thisStep {
		extDb.errors = &errorsWatch{
			Step:   thisStep,
			Low:    t,
			High:   t,
			Errors: []error{err},
		}
		return
	}

	// if exist
	if t < extDb.errors.Low {
		extDb.errors.Low = t
	}
	if t > extDb.errors.High {
		extDb.errors.High = t
	}
	extDb.errors.Errors = append(extDb.errors.Errors, err)

	k1 := math.Ceil((float64(extDb.errors.High-extDb.errors.Low) / float64(cHeartBeatWatchInterval)) * 100)
	k2 := math.Ceil(float64(len(extDb.errors.Errors)) / (float64(cHeartBeatWatchInterval) / float64(cReconnectBanTime)) * 100)

	if k1 >= cHeartBeatWatchPercent && k2 >= cHeartBeatWatchPercent {
		log.Warn("Database connection reached error limits ", *extDb.errors, " ", k1, " ", k2)
		select {
		case extDb.WarnChan <- extDb.errors.Errors:
		default:
			log.Warn("No one listener for db errors")
		}
	}
}

// GetConnection return original connection object for use other
// methods without reconnect
func (extDb *Database) GetConnection() (*sql.DB, error) {
	if db := extDb.getDb(); db != nil {
		return db, nil
	}
	return nil, ErrNotInitialized
}

// getDb safely getter for raw db field
func (extDb *Database) getDb() *sql.DB {
	extDb.access.RLock()
	db := extDb.db
	extDb.access.RUnlock()
	return db
}

// Prepare function with reconnect logic
func (extDb *Database) Prepare(query string) (*sql.Stmt, error) {
	err := extDb.checkStatus()
	if err != nil {
		return nil, err
	}
	db := extDb.getDb()
	if db == nil {
		return nil, ErrNotInitialized
	}
	stmt, err := db.Prepare(query)
	if err != nil {
		if !extDb.i.isConnectionError(err) {
			return nil, err
		}
		log.Info("Database prepare error ", err)
		errConn := extDb.Reconnect()
		if errConn == nil {
			db := extDb.getDb()
			if db == nil {
				return nil, ErrNotInitialized
			}
			stmt, err = db.Prepare(query)
			if err != nil {
				return nil, err
			}
			return stmt, nil
		}
		return nil, errConn
	}
	return stmt, nil
}

// checkStatus check database ready status and try reconnect if need
// return error if something wrong
func (extDb *Database) checkStatus() error {
	if atomic.LoadInt32(&extDb.isReady) == cDatabaseStateReconnect {
		return ErrReconInProcess
	}
	db := extDb.getDb()
	if atomic.LoadInt32(&extDb.isReady) == cDatabaseStateNotReady || db == nil {
		if err := extDb.Reconnect(); err != nil {
			return err
		}
	}
	return nil
}

// Exec function with reconnect logic
func (extDb *Database) Exec(query string, args ...interface{}) (sql.Result, error) {
	err := extDb.checkStatus()
	if err != nil {
		return nil, err
	}
	db := extDb.getDb()
	if db == nil {
		return nil, ErrNotInitialized
	}
	result, err := db.Exec(query, args...)
	if err != nil {
		if !extDb.i.isConnectionError(err) {
			return nil, err
		}
		log.Info("Database exec error ", err)
		errConn := extDb.Reconnect()
		if errConn == nil {
			db := extDb.getDb()
			if db == nil {
				return nil, ErrNotInitialized
			}
			result, err = db.Exec(query, args...)
			if err != nil {
				return nil, err
			}
			return result, nil
		}
		return nil, errConn
	}
	return result, nil
}

// Query function with reconnect logic
func (extDb *Database) Query(query string, args ...interface{}) (*sql.Rows, error) {
	err := extDb.checkStatus()
	if err != nil {
		return nil, err
	}
	db := extDb.getDb()
	if db == nil {
		return nil, ErrNotInitialized
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		if !extDb.i.isConnectionError(err) {
			return nil, err
		}
		log.Info("Database query error ", err)
		errConn := extDb.Reconnect()
		if errConn == nil {
			db := extDb.getDb()
			if db == nil {
				return nil, ErrNotInitialized
			}
			rows, err = db.Query(query, args...)
			if err != nil {
				return nil, err
			}
			return rows, nil
		}
		return nil, errConn
	}
	return rows, nil
}

// QueryRow function with reconnect logic
func (extDb *Database) QueryRow(query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := extDb.Query(query, args...)
	if err == nil && rows.Next() {
		return rows, nil
	}
	// close rows if some error happen and rows exist
	if rows != nil {
		rows.Close()
	}
	if err == nil {
		err = sql.ErrNoRows
	}
	return nil, err
}

// StoreBatch uploads multiple values in one request.
// Query should be like 'insert into table (a, b, c) values %s ...',
// where %s will be replaced with placeholders.
// 'itemsPerValue' is the numbers of fields to insert per 1 item.
// returns the number of affected rows and an error.
func StoreBatch(e Executor, query string, vals []interface{}, itemsPerValue int) (int64, error) {
	if len(vals) == 0 || itemsPerValue == 0 {
		return 0, fmt.Errorf("invalid number of values")
	}
	valuesStr := bytes.NewBufferString("")
	str := "(?"
	for j := 0; j < itemsPerValue-1; j++ {
		str += ",?"
	}
	str += ")"
	valuesCount := len(vals) / itemsPerValue
	for i := 0; i < valuesCount; i++ {
		valuesStr.WriteString(str)
		if i < valuesCount-1 {
			valuesStr.WriteString(",")
		}
	}
	sqlStr := fmt.Sprintf(query, valuesStr.String())
	res, err := e.Exec(sqlStr, vals...)
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

//StartTransaction start transaction and return TxConnection with error
func (extDb *Database) StartTransaction() (*TxConnection, error) {
	con, err := extDb.GetConnection()
	if err != nil {
		return nil, err
	}

	tx, err := con.Begin()
	if err != nil {
		return nil, err
	}
	if err = extDb.i.initTx(tx); err != nil {
		return nil, err
	}
	return &TxConnection{Tx: tx, Con: con}, nil
}

//Commit try to commit
func (txC *TxConnection) Commit() (err error) {
	return txC.Tx.Commit()
}

//Rollback try to rollback transaction
func (txC *TxConnection) Rollback() (err error) {
	return txC.Tx.Rollback()
}

// IsErrorDuplicateKey check if given error is duplicate key error
func IsErrorDuplicateKey(err error) bool {
	if isMySQLDupErr(err) {
		return true
	}
	if isPqDupErr(err) {
		return true
	}
	return false
}
