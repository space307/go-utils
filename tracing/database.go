package tracing

import (
	"context"
	"database/sql"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/space307/go-utils/database"
)

type Database struct {
	ExtDB *database.Database
}

func Init(driver string, config *database.Config) (*Database, error) {
	extDB, err := database.Init(driver, config)
	return &Database{ExtDB: extDB}, err
}

//execute http-request with tracing
func (d *Database) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	span, _ := d.createSpanFromContext(ctx, query)
	defer span.Finish()

	res, err := d.ExtDB.Exec(query, args...)

	if err != nil {
		span.LogFields(log.String("error", err.Error()))
	}

	return res, err
}

func (d *Database) QueryRow(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	span, _ := d.createSpanFromContext(ctx, query)
	defer span.Finish()

	rows, err := d.ExtDB.QueryRow(query, args...)
	if err != nil {
		span.LogFields(log.String("error", err.Error()))
	}

	return rows, err
}

func (d *Database) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	span, _ := d.createSpanFromContext(ctx, query)
	defer span.Finish()

	rows, err := d.ExtDB.Query(query, args...)
	if err != nil {
		span.LogFields(log.String("error", err.Error()))
	}

	return rows, err
}

func (d *Database) StartTransaction(ctx context.Context) (context.Context, *database.TxConnection, error) {
	span, ctx := d.createSpanFromContext(ctx, `startTransaction`)

	tx, err := d.ExtDB.StartTransaction()
	if err != nil {
		span.LogFields(log.String("error", err.Error()))
	}

	return opentracing.ContextWithSpan(ctx, span), tx, err
}

func (d *Database) Rollback(ctx context.Context, conn *database.TxConnection) error {
	span := opentracing.SpanFromContext(ctx)
	if span != nil {
		defer span.Finish()
	}

	err := conn.Tx.Rollback()
	if err != nil {
		span.LogFields(log.String("error", err.Error()))
	}

	return err
}

func (d *Database) Commit(ctx context.Context, conn *database.TxConnection) error {
	span := opentracing.SpanFromContext(ctx)
	if span != nil {
		defer span.Finish()
	}

	err := conn.Tx.Commit()
	if err != nil {
		span.LogFields(log.String("error", err.Error()))
	}

	return err
}

func (d *Database) ExecInsideTransaction(ctx context.Context, conn *database.TxConnection, query string, args ...interface{}) (sql.Result, error) {
	span, _ := d.createSpanFromContext(ctx, query)
	defer span.Finish()

	rows, err := conn.Tx.Exec(query, args...)
	if err != nil {
		span.LogFields(log.String("error", err.Error()))
	}

	return rows, err
}

func (d *Database) QueryInsideTransaction(ctx context.Context, conn *database.TxConnection, query string, args ...interface{}) (*sql.Rows, error) {
	span, _ := d.createSpanFromContext(ctx, query)
	defer span.Finish()

	rows, err := conn.Tx.Query(query, args...)
	if err != nil {
		span.LogFields(log.String("error", err.Error()))
	}

	return rows, err
}

func (d *Database) QueryRowInsideTransaction(ctx context.Context, conn *database.TxConnection, query string, args ...interface{}) *sql.Row {
	span, _ := d.createSpanFromContext(ctx, query)
	defer span.Finish()

	return conn.Tx.QueryRow(query, args...)
}

func (d *Database) createSpanFromContext(ctx context.Context, query string) (opentracing.Span, context.Context) {
	span, ctx := opentracing.StartSpanFromContext(ctx, query)

	ext.PeerService.Set(span, `db`)
	ext.PeerAddress.Set(span, d.ExtDB.GetConfig().Addr)
	ext.SpanKind.Set(span, ext.SpanKindRPCServerEnum)
	ext.DBStatement.Set(span, query)
	ext.DBType.Set(span, `sql`)
	ext.DBUser.Set(span, d.ExtDB.GetConfig().User)

	return span, ctx
}
