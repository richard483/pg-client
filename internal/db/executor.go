package db

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

var safeName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Executor struct {
	DSNBuilder       func(database string) string
	Timeout          time.Duration
	MaxRows          int
	MaxResponseBytes int
}

type Request struct {
	Database string `json:"db"`
	Schema   string `json:"schema"`
	Query    string `json:"query"`
}

type Execution struct {
	Database   string          `json:"database"`
	Schema     string          `json:"schema"`
	DurationMS int64           `json:"duration_ms"`
	Results    []Result        `json:"results"`
	Limits     Limits          `json:"limits"`
	Flags      Flags           `json:"flags"`
	Error      *ExecutionError `json:"error,omitempty"`
}

type Result struct {
	Index        int              `json:"index"`
	Columns      []Column         `json:"columns,omitempty"`
	Rows         []map[string]any `json:"rows,omitempty"`
	RowsRead     int              `json:"rows_read"`
	RowsIncluded int              `json:"rows_included"`
	CommandTag   string           `json:"command_tag"`
	RowsAffected int64            `json:"rows_affected"`
}

type Column struct {
	Name        string `json:"name"`
	DataTypeOID uint32 `json:"data_type_oid"`
	Format      int16  `json:"format"`
}

type Limits struct {
	MaxRows          int `json:"max_rows"`
	MaxResponseBytes int `json:"max_response_bytes"`
	TimeoutSeconds   int `json:"timeout_seconds"`
}

type Flags struct {
	RowsTruncated          bool `json:"rows_truncated"`
	ResponseBytesTruncated bool `json:"response_bytes_truncated"`
	HasMoreData            bool `json:"has_more_data"`
}

type ExecutionError struct {
	Message string `json:"message"`
}

func (e Executor) Execute(ctx context.Context, req Request) (out Execution) {
	start := time.Now()
	out = Execution{
		Database: req.Database,
		Schema:   req.Schema,
		Limits: Limits{
			MaxRows:          e.MaxRows,
			MaxResponseBytes: e.MaxResponseBytes,
			TimeoutSeconds:   int(e.Timeout.Seconds()),
		},
	}
	defer func() {
		out.DurationMS = time.Since(start).Milliseconds()
	}()

	if err := validateRequest(req); err != nil {
		out.Error = &ExecutionError{Message: err.Error()}
		return
	}

	execCtx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	conn, err := pgx.Connect(execCtx, e.DSNBuilder(req.Database))
	if err != nil {
		out.Error = &ExecutionError{Message: err.Error()}
		return
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(execCtx, "SET search_path TO "+pgx.Identifier{req.Schema}.Sanitize()); err != nil {
		out.Error = &ExecutionError{Message: err.Error()}
		return
	}

	reader := conn.PgConn().Exec(execCtx, req.Query)

	resultIndex := 0
	estimatedBytes := 0
	for reader.NextResult() {
		resultIndex++
		resultReader := reader.ResultReader()
		result := Result{Index: resultIndex}

		fields := resultReader.FieldDescriptions()
		result.Columns = make([]Column, len(fields))
		for i, field := range fields {
			result.Columns[i] = Column{
				Name:        field.Name,
				DataTypeOID: field.DataTypeOID,
				Format:      field.Format,
			}
		}

		for resultReader.NextRow() {
			result.RowsRead++
			values := resultReader.Values()
			if len(result.Rows) >= e.MaxRows {
				out.Flags.RowsTruncated = true
				out.Flags.HasMoreData = true
				continue
			}

			row := map[string]any{}
			for i, field := range fields {
				if values[i] == nil {
					row[field.Name] = nil
					continue
				}
				row[field.Name] = string(values[i])
			}

			rowSize := estimatedJSONSize(row)
			if estimatedBytes+rowSize > e.MaxResponseBytes {
				out.Flags.ResponseBytesTruncated = true
				out.Flags.HasMoreData = true
				continue
			}
			estimatedBytes += rowSize
			result.Rows = append(result.Rows, row)
			result.RowsIncluded = len(result.Rows)
		}

		commandTag, err := resultReader.Close()
		result.CommandTag = commandTag.String()
		result.RowsAffected = commandTag.RowsAffected()
		out.Results = append(out.Results, result)
		if err != nil {
			_ = reader.Close()
			out.Error = &ExecutionError{Message: err.Error()}
			return
		}
	}

	if err := reader.Close(); err != nil {
		out.Error = &ExecutionError{Message: err.Error()}
	}

	return
}

func validateRequest(req Request) error {
	if req.Database == "" {
		return errors.New("db is required")
	}
	if req.Schema == "" {
		return errors.New("schema is required")
	}
	if req.Query == "" {
		return errors.New("query is required")
	}
	if !safeName.MatchString(req.Database) {
		return errors.New("db must contain only letters, numbers, and underscores, and must not start with a number")
	}
	if !safeName.MatchString(req.Schema) {
		return errors.New("schema must contain only letters, numbers, and underscores, and must not start with a number")
	}
	return nil
}

func estimatedJSONSize(value any) int {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return len(raw)
}
