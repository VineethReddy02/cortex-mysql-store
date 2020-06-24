package mysql_store

import (
	"context"
	"fmt"
	"github.com/VineethReddy02/cortex-mysql-store/grpc"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (s *server) ListTables(context.Context, *empty.Empty) (*grpc.ListTablesResponse, error) {
	s.Logger.Info("listing the tables ")
	var tables []string
	r, err := s.Session.Query("show tables;")
	if err != nil {
		s.Logger.Error("failed in fetching the list of tables %s", zap.Error(err))
		return nil, errors.WithStack(err)
	}
	if r != nil {
		var tableName string
		for r.Next() {
			err = r.Scan(&tableName)
			if err != nil {
				s.Logger.Error("failed in scan the tables %s", zap.Error(err))
			}

			tables = append(tables, tableName)
		}
	}

	result := &grpc.ListTablesResponse{}
	result.TableNames = tables
	return result, nil
}

func (s *server) CreateTable(ctx context.Context, req *grpc.CreateTableRequest) (*empty.Empty, error) {
	s.Logger.Info("creating the table ", zap.String("Table Name", req.Desc.Name))
	_, err := s.Session.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			hash VARCHAR(250) NOT NULL,
			range01 VARCHAR(250) NOT NULL,
			value LONGBLOB,
			PRIMARY KEY (hash, range01)
		) DEFAULT CHARSET=utf8;`, req.Desc.Name))
	if err != nil {
		s.Logger.Error("failed to create the table %s", zap.Error(err))
	}
	return &empty.Empty{}, errors.WithStack(err)
}

func (s *server) DeleteTable(ctx context.Context, tableName *grpc.DeleteTableRequest) (*empty.Empty, error) {
	s.Logger.Info("deleting the table ", zap.String("Table Name", tableName.TableName))
	_, err := s.Session.Query(fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, tableName.TableName))
	if err != nil {
		s.Logger.Error("failed to delete the table %s", zap.Error(err))
	}
	return &empty.Empty{}, errors.WithStack(err)
}

func (s *server) DescribeTable(ctx context.Context, tableName *grpc.DescribeTableRequest) (*grpc.DescribeTableResponse, error) {
	s.Logger.Info("describing the table ", zap.String("Table Name", tableName.TableName))
	name := tableName.TableName
	return &grpc.DescribeTableResponse{
		Desc: &grpc.TableDesc{
			Name: name,
		},
		IsActive: true,
	}, nil
}

func (s *server) UpdateTable(context.Context, *grpc.UpdateTableRequest) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}
