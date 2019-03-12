package tracing

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/space307/go-utils/v3/database"
	"github.com/stretchr/testify/suite"
)

type testDBSuite struct {
	suite.Suite
	testServer *httptest.Server
	db         *Database
}

func (s *testDBSuite) TestSimpleQueryTracing() {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	beforeSpan := opentracing.GlobalTracer().StartSpan("to_inject").(*mocktracer.MockSpan)
	defer beforeSpan.Finish()

	beforeCtx := opentracing.ContextWithSpan(context.Background(), beforeSpan)

	// check Exec
	_, err := s.db.Exec(beforeCtx, `insert into t1(id, v) values(0, 1)`)
	s.NoError(err)

	finishedSpans := opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 1)

	finishedSpan := finishedSpans[0]
	s.Equal(`db`, finishedSpan.Tag(string(ext.PeerService)))
	s.Equal(`insert into t1(id, v) values(0, 1)`, finishedSpan.Tag(string(ext.DBStatement)))

	beforeContext := beforeSpan.Context().(mocktracer.MockSpanContext)
	s.Equal(beforeContext.SpanID, finishedSpan.ParentID)

	// check error Exec
	_, err = s.db.Exec(beforeCtx, `insert into t1(id, v) values(0, 1)`)
	s.Error(err)
	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 2)
	s.Len(finishedSpans[1].Logs(), 1)

	// check Query
	_, err = s.db.Query(beforeCtx, `SELECT * FROM t1`)
	s.NoError(err)
	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 3)

	// check error Query
	_, err = s.db.Query(beforeCtx, `SELECT unknown FROM t1`)
	s.Error(err)
	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 4)
	s.Len(finishedSpans[3].Logs(), 1)

	// check query row
	_, err = s.db.QueryRow(beforeCtx, `SELECT * FROM t1`)
	s.NoError(err)
	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 5)

	_, err = s.db.QueryRow(beforeCtx, `SELECT unknown FROM t1`)
	s.Error(err)
	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 6)
	s.Len(finishedSpans[5].Logs(), 1)

	_, err = s.db.Query(context.Background(), `SELECT * FROM t1`)
	s.NoError(err)
	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 7)
	s.Equal(0, finishedSpans[6].ParentID)
}

func (s *testDBSuite) TestTransactionTracing() {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	beforeSpan := opentracing.GlobalTracer().StartSpan("global").(*mocktracer.MockSpan)

	beforeCtx := opentracing.ContextWithSpan(context.Background(), beforeSpan)
	ctx, conn, err := s.db.StartTransaction(beforeCtx)
	s.NoError(err)
	_, err = s.db.ExecInsideTransaction(ctx, conn, `insert into t1(id, v) values(6, 6)`)
	s.NoError(err)
	_, err = s.db.ExecInsideTransaction(ctx, conn, `insert into t1(id, v) values(6, 6)`)
	s.Error(err)
	err = s.db.Rollback(ctx, conn)
	s.NoError(err)
	beforeSpan.Finish()

	finishedSpans := opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 4)
	s.Len(finishedSpans[1].Logs(), 1)
	s.Equal(finishedSpans[3].ParentID, 0)

	// Start success transaction
	opentracing.GlobalTracer().(*mocktracer.MockTracer).Reset()

	ctx, conn, err = s.db.StartTransaction(context.TODO())
	s.NoError(err)
	_, err = s.db.ExecInsideTransaction(ctx, conn, `insert into t1(id, v) values(7, 7)`)
	s.NoError(err)
	rows, err := s.db.QueryInsideTransaction(ctx, conn, `select * from t1`)
	s.NoError(err)
	rows.Close()
	var id int
	err = s.db.QueryRowInsideTransaction(ctx, conn, `select id from t1 WHERE id=$1`, 101).Scan(&id)
	s.EqualError(sql.ErrNoRows, err.Error())
	err = s.db.QueryRowInsideTransaction(ctx, conn, `select id from t1 WHERE id=$1`, 7).Scan(&id)
	s.NoError(err)

	err = s.db.Commit(ctx, conn)
	s.NoError(err)

	finishedSpans = opentracing.GlobalTracer().(*mocktracer.MockTracer).FinishedSpans()
	s.Len(finishedSpans, 5)
}

func (s *testDBSuite) SetupSuite() {
	var err error
	cfg := &database.Config{
		Addr:     "127.0.0.1:5432",
		User:     "postgres",
		Database: "db_test",
		SSLMode:  "disable",
	}
	s.db, err = Init("postgres", cfg)
	s.NoError(err)

	_, err = s.db.GetExtDatabase().Exec("DROP TABLE t1;")
	_, err = s.db.GetExtDatabase().Exec("CREATE TABLE IF NOT EXISTS t1(id integer,v integer,CONSTRAINT t1_pkey PRIMARY KEY (id),CONSTRAINT t1_v_key UNIQUE (v));")
	s.Require().NoError(err)

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)
}

func (s *testDBSuite) TearDownSuite() {}

func TestTracingDBSuite(t *testing.T) {
	suite.Run(t, new(testDBSuite))
}
