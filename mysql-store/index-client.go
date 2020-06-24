package mysql_store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/VineethReddy02/cortex-mysql-store/grpc"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (s *server) WriteIndex(ctx context.Context, batch *grpc.WriteIndexRequest) (*empty.Empty, error) {
	for i, entry := range batch.Writes {
		s.Logger.Info("performing batch write. ", zap.String("Table name ", batch.Writes[i].TableName))
		_, err := s.Session.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (hash, range01, value) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE value=VALUES(value)",
			entry.TableName), entry.HashValue, entry.RangeValue, entry.Value)
		if err != nil {
			s.Logger.Error("failed to perform batch write ", zap.Error(err))
			return &empty.Empty{}, errors.WithStack(err)
		}
	}

	return &empty.Empty{}, nil
}

func (s *server) DeleteIndex(ctx context.Context, deletes *grpc.DeleteIndexRequest) (*empty.Empty, error) {
	for _, entry := range deletes.Deletes {
		_, err := s.Session.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE hash = '%s' and range = '%v'",
			entry.TableName, entry.HashValue, entry.RangeValue))
		if err != nil {
			s.Logger.Error("failed to delete index", zap.Error(err))
			return &empty.Empty{}, errors.WithStack(err)
		}
	}

	return &empty.Empty{}, nil
}

func (s *server) QueryIndex(query *grpc.QueryIndexRequest, queryStreamer grpc.GrpcStore_QueryIndexServer) error {
	s.Logger.Info("performing Query Pages ", zap.String("table name ", query.TableName))
	var rows *sql.Rows
	var err error
	switch {
	case len(query.RangeValuePrefix) > 0 && query.ValueEqual == nil:
		rows, err = s.Session.Query(fmt.Sprintf("SELECT range01, value FROM %s WHERE hash = ? AND range01 >= ? AND range01 < ?",
			query.TableName), query.HashValue, query.RangeValuePrefix, append(query.RangeValuePrefix, '\xff'))
	case len(query.RangeValuePrefix) > 0 && query.ValueEqual != nil:
		rows, err = s.Session.Query(fmt.Sprintf("SELECT range01, value FROM %s WHERE hash = ? AND range01 >= ? AND range01 < ? AND value = ?",
			query.TableName), query.HashValue, query.RangeValuePrefix, append(query.RangeValuePrefix, '\xff'), query.ValueEqual)
	case len(query.RangeValueStart) > 0 && query.ValueEqual == nil:
		rows, err = s.Session.Query(fmt.Sprintf("SELECT range01, value FROM %s WHERE hash = ? AND range01 >= ?",
			query.TableName), query.HashValue, query.RangeValueStart)
	case len(query.RangeValueStart) > 0 && query.ValueEqual != nil:
		rows, err = s.Session.Query(fmt.Sprintf("SELECT range01, value FROM %s WHERE hash = ? AND range01 >= ? AND value = ?",
			query.TableName), query.HashValue, query.RangeValueStart, query.ValueEqual)
	case query.ValueEqual == nil:
		rows, err = s.Session.Query(fmt.Sprintf("SELECT range01, value FROM %s WHERE hash = ?",
			query.TableName), query.HashValue)
	case query.ValueEqual != nil:
		rows, err = s.Session.Query(fmt.Sprintf("SELECT range01, value FROM %s WHERE hash = ? value = ?",
			query.TableName), query.HashValue, query.ValueEqual)
	}
	if err != nil {
		s.Logger.Error("failed to perform index query in query pages", zap.Error(err))
		return err
	}

	b1 := &grpc.QueryIndexResponse{
		Rows: []*grpc.Row{},
	}
	for rows.Next() {
		b := &grpc.Row{}
		err = rows.Scan(&b.RangeValue, &b.Value)
		if err != nil {
			s.Logger.Error("failed to scan row in query pages", zap.Error(err))
			return err
		}

		b1.Rows = append(b1.Rows, b)
	}

	// you can add custom logic here to break rows and send as stream instead of sending all at once.
	err = queryStreamer.Send(b1)
	if err != nil {
		s.Logger.Error("Unable to stream the results")
		return err
	}

	return nil
}
