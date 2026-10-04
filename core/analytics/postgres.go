package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/artpar/apigate/adapters/postgres"
	"github.com/google/uuid"
)

// PostgresStore implements Analytics with PostgreSQL backend.
type PostgresStore struct {
	db        *postgres.DB
	done      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once

	// Configuration
	batchSize     int
	flushInterval time.Duration
	costCalc      CostCalculator
}

// PostgresConfig configures the PostgreSQL analytics store.
type PostgresConfig struct {
	// BatchSize is the number of events to batch before writing.
	BatchSize int

	// FlushInterval is the maximum time between flushes.
	FlushInterval time.Duration

	// BufferSize is the size of the in-memory event buffer.
	BufferSize int

	// CostCalculator computes cost for events.
	CostCalculator CostCalculator
}

// DefaultPostgresConfig returns sensible defaults.
func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		BatchSize:      100,
		FlushInterval:  time.Second,
		BufferSize:     10000,
		CostCalculator: NewDefaultCostCalculator(),
	}
}

// NewPostgresStore creates a new PostgreSQL-backed analytics store.
func NewPostgresStore(db *sql.DB, cfg PostgresConfig) (*PostgresStore, error) {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 10000
	}
	if cfg.CostCalculator == nil {
		cfg.CostCalculator = NewDefaultCostCalculator()
	}

	s := &PostgresStore{
		db:            &postgres.DB{DB: db},
		done:          make(chan struct{}),
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		costCalc:      cfg.CostCalculator,
	}

	// Start background flusher
	s.wg.Add(1)
	go s.flusher()

	return s, nil
}

// createTable creates the analytics table.
// Record commits to the durable outbox before returning. Consumers retry failures
// without dropping the batch; monetary usage lives in the separate billing outbox.
func (s *PostgresStore) Record(event Event) {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	b, err := json.Marshal(event)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = s.db.ExecContext(ctx, "INSERT INTO analytics_outbox(id,payload) VALUES(?,?::jsonb) ON CONFLICT DO NOTHING", event.ID, string(b))
	}
	if err != nil {
		slog.Error("analytics outbox persistence failed", "error", err)
	}
}
func (s *PostgresStore) RecordAsync(event Event) { s.Record(event) }
func (s *PostgresStore) Flush(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT id,payload FROM analytics_outbox WHERE processed_at IS NULL ORDER BY created_at LIMIT ? FOR UPDATE SKIP LOCKED", s.batchSize)
	if err != nil {
		return err
	}
	var events []Event
	for rows.Next() {
		var event Event
		var id string
		var b []byte
		if err = rows.Scan(&id, &b); err == nil {
			err = json.Unmarshal(b, &event)
		}
		if err != nil {
			rows.Close()
			return err
		}
		event.ID = id
		events = append(events, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range events {
		success := 0
		if e.Success {
			success = 1
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO analytics(id,timestamp,channel,module,action,record_id,user_id,api_key_id,remote_ip,duration_ns,memory_bytes,request_bytes,response_bytes,success,status_code,error,cost_units) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, e.ID, e.Timestamp.UTC().Format(time.RFC3339Nano), e.Channel, e.Module, e.Action, e.RecordID, e.UserID, e.APIKeyID, e.RemoteIP, e.DurationNS, e.MemoryBytes, e.RequestBytes, e.ResponseBytes, success, e.StatusCode, e.Error, s.costCalc.Calculate(e))
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE analytics_outbox SET processed_at=CURRENT_TIMESTAMP WHERE id=?", e.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *PostgresStore) flusher() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := s.Flush(ctx)
			cancel()
			if err != nil {
				slog.Error("analytics consumer retry pending", "error", err)
			}
		}
	}
}

// Write writes events to storage.
func (s *PostgresStore) Write(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO analytics (
			id, timestamp, channel, module, action, record_id,
			user_id, api_key_id, remote_ip,
			duration_ns, memory_bytes, request_bytes, response_bytes,
			success, status_code, error, cost_units
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
		if e.Timestamp.IsZero() {
			e.Timestamp = time.Now()
		}

		cost := s.costCalc.Calculate(e)

		successInt := 0
		if e.Success {
			successInt = 1
		}

		_, err := stmt.ExecContext(ctx,
			e.ID, e.Timestamp.UTC().Format(time.RFC3339Nano),
			e.Channel, e.Module, e.Action, e.RecordID,
			e.UserID, e.APIKeyID, e.RemoteIP,
			e.DurationNS, e.MemoryBytes, e.RequestBytes, e.ResponseBytes,
			successInt, e.StatusCode, e.Error, cost,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Query retrieves events matching the options.
func (s *PostgresStore) Query(ctx context.Context, opts QueryOptions) ([]Event, int64, error) {
	var conditions []string
	var args []any

	if !opts.Start.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, opts.Start.UTC().Format(time.RFC3339Nano))
	}
	if !opts.End.IsZero() {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, opts.End.UTC().Format(time.RFC3339Nano))
	}
	if opts.Channel != "" {
		conditions = append(conditions, "channel = ?")
		args = append(args, opts.Channel)
	}
	if opts.Module != "" {
		conditions = append(conditions, "module = ?")
		args = append(args, opts.Module)
	}
	if opts.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, opts.Action)
	}
	if opts.UserID != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, opts.UserID)
	}
	if opts.APIKeyID != "" {
		conditions = append(conditions, "api_key_id = ?")
		args = append(args, opts.APIKeyID)
	}
	if opts.Success != nil {
		if *opts.Success {
			conditions = append(conditions, "success = 1")
		} else {
			conditions = append(conditions, "success = 0")
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	var total int64
	countQuery := "SELECT COUNT(*) FROM analytics " + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Order - whitelist allowed columns to prevent SQL injection
	allowedOrderCols := map[string]bool{
		"timestamp":      true,
		"duration_ns":    true,
		"memory_bytes":   true,
		"request_bytes":  true,
		"response_bytes": true,
		"module":         true,
		"action":         true,
		"channel":        true,
	}
	orderBy := "timestamp"
	if opts.OrderBy != "" && allowedOrderCols[opts.OrderBy] {
		orderBy = opts.OrderBy
	}
	order := "DESC"
	if !opts.OrderDesc {
		order = "ASC"
	}

	// Limit/offset
	limit := 100
	if opts.Limit > 0 {
		limit = opts.Limit
	}

	query := fmt.Sprintf(`
		SELECT id, timestamp, channel, module, action, record_id,
			user_id, api_key_id, remote_ip,
			duration_ns, memory_bytes, request_bytes, response_bytes,
			success, status_code, error
		FROM analytics %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, where, orderBy, order)

	args = append(args, limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var ts string
		var success int
		var recordID, userID, apiKeyID, remoteIP, errMsg sql.NullString
		var statusCode sql.NullInt64

		err := rows.Scan(
			&e.ID, &ts, &e.Channel, &e.Module, &e.Action, &recordID,
			&userID, &apiKeyID, &remoteIP,
			&e.DurationNS, &e.MemoryBytes, &e.RequestBytes, &e.ResponseBytes,
			&success, &statusCode, &errMsg,
		)
		if err != nil {
			return nil, 0, err
		}

		e.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		e.Success = success == 1
		e.RecordID = recordID.String
		e.UserID = userID.String
		e.APIKeyID = apiKeyID.String
		e.RemoteIP = remoteIP.String
		e.StatusCode = int(statusCode.Int64)
		e.Error = errMsg.String

		events = append(events, e)
	}

	return events, total, rows.Err()
}

// Aggregate returns summarized analytics.
func (s *PostgresStore) Aggregate(ctx context.Context, opts AggregateOptions) ([]Summary, error) {
	var conditions []string
	var args []any

	if !opts.Start.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, opts.Start.UTC().Format(time.RFC3339Nano))
	}
	if !opts.End.IsZero() {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, opts.End.UTC().Format(time.RFC3339Nano))
	}
	if opts.Channel != "" {
		conditions = append(conditions, "channel = ?")
		args = append(args, opts.Channel)
	}
	if opts.Module != "" {
		conditions = append(conditions, "module = ?")
		args = append(args, opts.Module)
	}
	if opts.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, opts.Action)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Build GROUP BY
	var groupCols []string
	var selectCols []string

	for _, g := range opts.GroupBy {
		switch g {
		case "module":
			groupCols = append(groupCols, "module")
			selectCols = append(selectCols, "module")
		case "action":
			groupCols = append(groupCols, "action")
			selectCols = append(selectCols, "action")
		case "channel":
			groupCols = append(groupCols, "channel")
			selectCols = append(selectCols, "channel")
		}
	}

	// Time period grouping
	periodExpr := ""
	switch opts.Period {
	case "minute":
		periodExpr = "to_char(timestamp::timestamptz AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI')"
	case "hour":
		periodExpr = "to_char(timestamp::timestamptz AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24')"
	case "day":
		periodExpr = "to_char(timestamp::timestamptz AT TIME ZONE 'UTC', 'YYYY-MM-DD')"
	}

	if periodExpr != "" {
		groupCols = append(groupCols, periodExpr)
		selectCols = append(selectCols, periodExpr+" as period")
	}

	groupBy := ""
	if len(groupCols) > 0 {
		groupBy = "GROUP BY " + strings.Join(groupCols, ", ")
	}

	selectPart := strings.Join(selectCols, ", ")
	if selectPart != "" {
		selectPart += ","
	}

	query := fmt.Sprintf(`
		SELECT %s
			COUNT(*) as total_requests,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_requests,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) as error_requests,
			CAST(COALESCE(AVG(duration_ns), 0) AS BIGINT) as avg_duration_ns,
			MIN(duration_ns) as min_duration_ns,
			MAX(duration_ns) as max_duration_ns,
			SUM(memory_bytes) as total_memory_bytes,
			SUM(request_bytes) as total_request_bytes,
			SUM(response_bytes) as total_response_bytes,
			SUM(cost_units) as cost_units,
			MIN(timestamp) as start_time,
			MAX(timestamp) as end_time
		FROM analytics %s %s
		ORDER BY start_time DESC
	`, selectPart, where, groupBy)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []Summary
	for rows.Next() {
		var sum Summary
		var startStr, endStr string
		var module, action, channel, period sql.NullString

		// Build scan targets based on groupBy
		scanTargets := make([]any, 0)
		for _, g := range opts.GroupBy {
			switch g {
			case "module":
				scanTargets = append(scanTargets, &module)
			case "action":
				scanTargets = append(scanTargets, &action)
			case "channel":
				scanTargets = append(scanTargets, &channel)
			}
		}
		if opts.Period != "" {
			scanTargets = append(scanTargets, &period)
		}

		scanTargets = append(scanTargets,
			&sum.TotalRequests, &sum.SuccessRequests, &sum.ErrorRequests,
			&sum.AvgDurationNS, &sum.MinDurationNS, &sum.MaxDurationNS,
			&sum.TotalMemoryBytes, &sum.TotalRequestBytes, &sum.TotalResponseBytes,
			&sum.CostUnits, &startStr, &endStr,
		)

		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}

		sum.Channel = channel.String
		sum.Module = module.String
		sum.Action = action.String
		sum.Period = period.String
		sum.Start, _ = time.Parse(time.RFC3339Nano, startStr)
		sum.End, _ = time.Parse(time.RFC3339Nano, endStr)

		summaries = append(summaries, sum)
	}

	return summaries, rows.Err()
}

// Delete removes events older than the given time.
func (s *PostgresStore) Delete(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM analytics WHERE timestamp < ?",
		before.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Close shuts down the store.
func (s *PostgresStore) Close() error {
	s.closeOnce.Do(func() { close(s.done) })
	s.wg.Wait()
	return nil
}
