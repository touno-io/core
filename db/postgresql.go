package db

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

const (
	PGHOST     = "PG_HOST"
	PGPORT     = "PG_PORT"
	PGUSER     = "PG_USER"
	PGPASSWORD = "PG_PASS"
	PGDATABASE = "PG_DBNAME"
	PGCACHE    = "PG_DBCACHE"
	PGSSLMODE  = "PG_SSLMODE"
	PGLIFETIME = "PG_LIFETIME"
	PGMAXIDLE  = "PG_MAXIDLE"
	PGMAXCONN  = "PG_MAXCONN"
)

var ErrNoRows error = sql.ErrNoRows

func getSSLMode() string {

	sslmode := os.Getenv(PGSSLMODE)
	if strings.Contains(os.Getenv(PGSSLMODE), "") {
		sslmode = "disable"
	}
	return sslmode
}

func getDSN(appName string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s application_name='%s'",
		os.Getenv(PGHOST), os.Getenv(PGPORT), os.Getenv(PGUSER), os.Getenv(PGPASSWORD), os.Getenv(PGDATABASE), getSSLMode(), appName)
}

type PGClient struct {
	DB  *sql.DB
	ctx *context.Context
}

type PGRow map[string]string
type PGRecord []PGRow

type PGTx struct {
	Closed bool
	tx     *sql.Tx
	ctx    *context.Context
}

type PGNotify struct {
	ln   *pq.Listener
	fail chan error
}

func (pg *PGNotify) Ping() error {
	return pg.ln.Ping()
}
func (pg *PGClient) CreateChannel(appTitle string) (*PGNotify, error) {
	n := &PGNotify{fail: make(chan error, 2)}

	n.ln = pq.NewListener(getDSN(appTitle), 5*time.Second, time.Minute, func(e pq.ListenerEventType, err error) {
		if err != nil {
			Errorf("Listen:: %s", err)
		}
		if e == pq.ListenerEventConnectionAttemptFailed {
			n.fail <- err
		} else {
			n.fail <- nil
		}
	})
	err := <-n.fail
	Infof("'listen::%s/%s' Consumed", os.Getenv(PGHOST), os.Getenv(PGDATABASE))
	return n, err
}

func (pg *PGNotify) Listen(channelName string, eventCallback func(e *pq.Notification)) error {
	Infof("LISTEN channel '%s'", channelName)
	if err := pg.ln.Listen(channelName); err != nil {
		pg.ln.Close()
		return err
	}

	go func() {
		for {
			select {
			case e := <-pg.ln.Notify:
				if e == nil {
					continue
				}
				eventCallback(e)
			case <-time.After(time.Minute * 5):
				go pg.ln.Ping()
			}
		}
	}()

	return nil
}

func (pg *PGNotify) Close() error {
	close(pg.fail)
	return pg.ln.Close()
}

func (pg *PGClient) Connect(c *context.Context, appTitle string) {
	var err error
	pg.ctx = c

	pg.DB, err = sql.Open("postgres", getDSN(appTitle))
	if err != nil {
		Trace.Fatal("Postgres:: Open", err)
	}

	if os.Getenv(PGLIFETIME) != "" {
		lifeTimeSecond, err := strconv.ParseInt(os.Getenv(PGLIFETIME), 0, 64)
		if err != nil {
			Error("ENV::PGLIFETIME ParseInt", err)
		}
		maxIdle, err := strconv.ParseInt(os.Getenv(PGMAXIDLE), 0, 32)
		if err != nil {
			Error("ENV::PGMAXIDLE ParseInt", err)
		}
		maxConn, err := strconv.ParseInt(os.Getenv(PGMAXCONN), 0, 32)
		if err != nil {
			Error("ENV::PGMAXCONN ParseInt", err)
		}

		pg.DB.SetConnMaxLifetime(time.Second * time.Duration(lifeTimeSecond))
		pg.DB.SetMaxIdleConns(int(maxIdle))
		pg.DB.SetMaxOpenConns(int(maxConn))
	}

	err = pg.DB.PingContext(*pg.ctx)
	if err != nil {
		Trace.Fatal("Postgres:: PingContext", err)
	}

	Infof("'query::%s/%s' Connected ", os.Getenv(PGHOST), os.Getenv(PGDATABASE))
}

func (pg *PGClient) Close() error {
	return pg.DB.Close()
}

func (pg PGRow) ToByte(name string) []byte {
	return []byte(pg[name])
}

func (pg PGRow) ToBoolean(name string) bool {
	if pg[name] == "" {
		return false
	}

	data, err := strconv.ParseBool(pg[name])
	if err != nil {
		Errorf("PGRow.ToBoolean('%s'): %s", name, err)
	}
	return data
}
func (pg PGRow) ToInt64(name string) int64 {
	data, err := strconv.ParseInt(pg[name], 0, 64)
	if err != nil {
		Errorf("PGRow.ToInt64('%s', 0, 64): %s", name, err)
	}
	return data
}
func (pg PGRow) ToFloat64(name string) float64 {
	data, err := strconv.ParseFloat(pg[name], 64)
	if err != nil {
		Errorf("PGRow.ToFloat64('%s', 64): %s", name, err)
	}
	return data
}

func (pg PGRow) ToTime(name string) time.Time {
	data, err := time.Parse(time.RFC3339Nano, pg[name])
	if err != nil {
		Errorf("PGRow.ToTime('%s'): %s", name, err)
	}
	return data
}

const (
	LevelDefault sql.IsolationLevel = iota
	LevelReadUncommitted
	LevelReadCommitted
	LevelWriteCommitted
	LevelRepeatableRead
	LevelSnapshot
	LevelSerializable
	LevelLinearizable
)

func (pg *PGClient) Begin(level sql.IsolationLevel) (*PGTx, error) {
	// defer EstimatedPrint(time.Now(), fmt.Sprintf("Begin: %+v", pg.ctx))
	stx, err := pg.DB.BeginTx(*pg.ctx, &sql.TxOptions{Isolation: level})

	pgx := PGTx{tx: stx, ctx: pg.ctx}
	return &pgx, err
}

func (stx *PGTx) Commit() error {
	stx.Closed = true
	return stx.tx.Commit()
}

func (stx *PGTx) Rollback() error {
	stx.Closed = true
	return stx.tx.Rollback()
}

func (stx *PGTx) QueryOne(query string, args ...any) (PGRow, error) {
	rows, err := sctxQuery(stx.tx, stx.ctx, false, query, args...)

	if err != nil {
		return nil, fmt.Errorf("QueryOne::%s", err.Error())
	}
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	defer rows.Close()
	return fetchRow(rows)
}

func (stx *PGTx) QueryOnePrint(query string, args ...any) (PGRow, error) {
	rows, err := sctxQuery(stx.tx, stx.ctx, true, query, args...)

	if err != nil {
		return nil, fmt.Errorf("QueryOne::%s", err.Error())
	}
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	defer rows.Close()
	return fetchRow(rows)
}

func (stx *PGTx) Query(query string, args ...any) (*sql.Rows, error) {
	return sctxQuery(stx.tx, stx.ctx, false, query, args...)
}

func (stx *PGTx) QueryPrint(query string, args ...any) (*sql.Rows, error) {
	return sctxQuery(stx.tx, stx.ctx, true, query, args...)
}

func (stx *PGTx) Execute(query string, args ...any) error {
	return sctxExecute(stx.tx, stx.ctx, false, query, args...)
}

func (stx *PGTx) ExecutePrint(query string, args ...any) error {
	return sctxExecute(stx.tx, stx.ctx, true, query, args...)
}

func (stx *PGTx) FetchRow(rows *sql.Rows) (PGRow, error) {
	return fetchRow(rows)
}

func (stx *PGTx) FetchAll(rows *sql.Rows) (PGRecord, error) {
	result := []PGRow{}
	for rows.Next() {
		data, err := stx.FetchRow(rows)
		if err != nil {
			return PGRecord{}, nil
		}

		result = append(result, data)
	}
	return result, nil
}
func (stx *PGTx) FetchOneColumn(rows *sql.Rows, columnName string) (SubSet, error) {
	result := SubSet{}
	for rows.Next() {
		data, err := stx.FetchRow(rows)
		if err != nil {
			return SubSet{}, nil
		}

		result = append(result, data[columnName])
	}
	return result, nil
}

func (row PGRecord) Find(columnName string, compareValue string) bool {
	for i := 0; i < len(row); i++ {
		if row[i][columnName] == compareValue {
			return true
		}
	}
	return false
}

func fetchRow(rows *sql.Rows) (PGRow, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("FetchRow::Columns::%s", err)
	}

	resultMap := make(PGRow)
	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	err = rows.Scan(pointers...)
	if err == sql.ErrNoRows {
		return resultMap, fmt.Errorf("FetchRow::ErrNoRows: %s", err)
	} else if err != nil {
		return nil, fmt.Errorf("FetchRow::Scan: %s", err)
	}

	for i, val := range values {
		if reflect.TypeOf(val) == nil {
			resultMap[columns[i]] = ""
			continue
		}
		switch reflect.TypeOf(val).String() {
		case "int64":
			resultMap[columns[i]] = fmt.Sprint(val.(int64))
		case "float64":
			resultMap[columns[i]] = fmt.Sprint(val.(float64))
		case "string":
			resultMap[columns[i]] = val.(string)
		case "[]uint8":
			resultMap[columns[i]] = string(val.([]uint8))
		case "bool":
			resultMap[columns[i]] = fmt.Sprintf("%t", val.(bool))
		case "time.Time":
			resultMap[columns[i]] = val.(time.Time).Format(time.RFC3339Nano)
		default:
			Errorf("Reflect TypeOf: %s ", reflect.TypeOf(val).String())
			resultMap[columns[i]] = ""
		}
	}
	return resultMap, nil
}

func sctxQuery(pgstx *sql.Tx, pgctx *context.Context, envDebug bool, query string, args ...any) (*sql.Rows, error) {
	elapsed := time.Now()
	if envDebug {
		defer sqlQuery(elapsed, query, args...)
	}
	defer estimatedPrint(elapsed, "Query")

	rows, err := pgstx.QueryContext(*pgctx, query, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func sctxExecute(pgstx *sql.Tx, pgctx *context.Context, envDebug bool, query string, args ...any) error {
	elapsed := time.Now()
	if envDebug {
		defer sqlQuery(elapsed, query, args...)
	}

	defer estimatedPrint(elapsed, "Execute")

	_, err := pgstx.ExecContext(*pgctx, query, args...)
	if err != nil {
		return err
	}
	return nil
}

func sqlQuery(elapsed time.Time, query string, args ...any) {
	for i, arg := range args {
		rgx := regexp.MustCompile(fmt.Sprintf(`\$%d`, i+1))
		query = rgx.ReplaceAllString(query, "'"+arg.(string)+"'")
	}
	logNone.Printf("[Query]\n")
	lead := 0
	for i, line := range strings.Split(strings.ReplaceAll(query, "\r\n", "\n"), "\n") {
		if i < 2 && lead == 0 {
			lead = leadingSpace(line)
		}
		if len(line) > lead && lead > 1 {
			line = line[lead-2:]
		}
		logNone.Println(strings.ReplaceAll(line, "\t", "  "))
	}
	logNone.Printf("\nElapsed time %d ms estimated.", estimated(elapsed))
	logNone.Printf("\n[Query]")
}

func leadingSpace(line string) int {
	count := 0
	for _, v := range line {
		if v == ' ' || v == '\t' {
			count++
		} else {
			break
		}
	}
	return count
}

type SubSet []string

func (s *SubSet) ToParam() string {
	return fmt.Sprintf("{%s}", strings.Join(*s, ","))
}
func (s *SubSet) Find(val string) int {
	for ix, v := range *s {
		if v == val {
			return ix
		}
	}
	return len(*s)
}

func estimated(start time.Time) int {
	duration, _ := elapsedDuration(start)
	return int(float64(duration.Microseconds()) / 1000)
}

func estimatedPrint(start time.Time, name string, ctx ...*fiber.Ctx) {
	if os.Getenv(DEBUG) == "false" && os.Getenv(ENV) == "production" {
		return
	}
	_, elapsed := elapsedDuration(start)

	pc, _, _, _ := runtime.Caller(1)
	funcObj := runtime.FuncForPC(pc)
	if name == "" {
		runtimeFunc := regexp.MustCompile(`^.*\.(.*)$`)
		name = runtimeFunc.ReplaceAllString(funcObj.Name(), "$1")
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// Debugf("%s # %s estimated. | alloc: %vMiB (%vMiB), sys: %vMiB, gc: %vMiB", name, elapsed, bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)

	if len(ctx) != 0 && ctx[0] != nil {
		ctx[0].Append("Server-Timing", fmt.Sprintf("app;dur=%v", elapsed))
	}
	Debugf("%s # %s estimated.", name, elapsed)
}

func elapsedDuration(start time.Time) (time.Duration, string) {
	duration := time.Since(start)

	elapsed := ""
	if duration.Nanoseconds() < 1000 {
		elapsed = fmt.Sprintf("%dns", duration.Nanoseconds())
	} else if duration.Microseconds() < 1000 {
		elapsed = fmt.Sprintf("%0.3fμs", round(float64(duration.Nanoseconds())/1000, 2))
	} else if duration.Milliseconds() < 1000 {
		elapsed = fmt.Sprintf("%0.3fms", round(float64(duration.Microseconds())/1000, 2))
	} else if duration.Seconds() < 60 {
		elapsed = fmt.Sprintf("%0.3fms", round(float64(duration.Microseconds())/1000, 2))
	} else {
		elapsed = fmt.Sprintf("%0.3fm", round(float64(duration.Seconds()/60), 2))
	}
	return duration, elapsed
}

// round math round decimal
func round(n float64, m float64) float64 {
	return math.Round(n*math.Pow(10, m)) / math.Pow(10, m)
}
