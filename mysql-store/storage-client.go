package mysql_store

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"github.com/VineethReddy02/cortex-mysql-store/grpc"
	"github.com/golang/protobuf/ptypes/empty"
	"strconv"
	"time"

	"github.com/cortexproject/cortex/pkg/chunk"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Config for a StorageClient
type Config struct {
	Addresses                string        `yaml:"addresses,omitempty"`
	GrpcServerPort           int           `yaml:"http_listen_port,omitempty"`
	Port                     int           `yaml:"port,omitempty"`
	Database                 string        `yaml:"database,omitempty"`
	DBUser                   string        `yaml:"dbuser,omitempty"`
	DBPassword               string        `yaml:"dbpassword,omitempty"`
	Consistency              string        `yaml:"consistency,omitempty"`
	ReplicationFactor        int           `yaml:"replication_factor,omitempty"`
	DisableInitialHostLookup bool          `yaml:"disable_initial_host_lookup,omitempty"`
	SSL                      bool          `yaml:"SSL,omitempty"`
	HostVerification         bool          `yaml:"host_verification,omitempty"`
	CAPath                   string        `yaml:"CA_path,omitempty"`
	Auth                     bool          `yaml:"auth,omitempty"`
	Username                 string        `yaml:"username,omitempty"`
	Password                 string        `yaml:"password,omitempty"`
	Timeout                  time.Duration `yaml:"timeout,omitempty"`
	ConnectTimeout           time.Duration `yaml:"connect_timeout,omitempty"`
}

// RegisterFlags adds the flags required to config this to the given FlagSet
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.Addresses, "mysql.addresses", "", "Comma-separated hostnames or IPs of MySQL instances.")
	f.IntVar(&cfg.Port, "mysql.port", 3306, "Port that mysql is running on")
	f.IntVar(&cfg.GrpcServerPort, "grpc.http_listen_port", 9966, "Port on which grpc mysql store should listen.")
	f.StringVar(&cfg.Database, "mysql.database", "", "DB to use in mysql.")
	f.StringVar(&cfg.DBUser, "mysql.dbuser", "", "DB user to use in mysql.")
	f.StringVar(&cfg.DBPassword, "mysql.dbpassword", "", "DB password to use in mysql.")
}

type server struct {
	Cfg       Config             `yaml:"cfg,omitempty"`
	SchemaCfg chunk.SchemaConfig `yaml:"schema_cfg,omitempty"`
	Session   *sql.DB            `yaml:"-"`
	Logger    *zap.Logger
}

// NewStorageClient returns a new StorageClient.
func NewStorageClient(cfg Config, schemaCfg chunk.SchemaConfig) (*server, error) {
	logger, _ := zap.NewProduction()
	client := &server{
		Cfg:       cfg,
		SchemaCfg: schemaCfg,
		Logger:    logger,
	}

	err := client.session()
	if err != nil {
		return nil, errors.WithStack(err)
	}


	return client, nil
}

func (s *server) session() error {
	dataSourceName := s.Cfg.Username + ":" + s.Cfg.Password + "@tcp(" + s.Cfg.Addresses + ":" + strconv.Itoa(s.Cfg.Port) + ")/"

	// initialise the conn with mysql-store
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		s.Logger.Error("failed to establish connection with mysql", zap.Error(err))
		return err
	}

	// create db if doesn't exist
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", s.Cfg.Database))
	if err != nil {
		s.Logger.Error("failed to create database ", zap.Error(err))
		return err
	}

	// override the previous mysql-store connection with db connection
	db, err = sql.Open("mysql", dataSourceName+s.Cfg.Database)
	if err != nil {
		s.Logger.Error("failed to establish connection with mysql database ", zap.Error(err))
		return err
	}

	// switch db context
	_, err = db.Exec(fmt.Sprintf("USE %s", s.Cfg.Database))
	if err != nil {
		s.Logger.Error("failed to switch db context in db ", zap.Error(err))
		return err
	}

	// verify db connection
	err = db.Ping()
	if err != nil {
		s.Logger.Error("failed to ping mysql database ", zap.Error(err))
		return err
	}

	s.Session = db

	return nil
}

// PutChunks implements chunk.ObjectClient.
func (s *server) PutChunks(ctx context.Context, chunks *grpc.PutChunksRequest) (*empty.Empty, error) {
	for _, chunkInfo := range chunks.Chunks {
		s.Logger.Info("performing put chunks.", zap.String("table name", chunkInfo.TableName))
		_, err := s.Session.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (hash, range01, value) VALUES (?, 0x00, ?) ON DUPLICATE KEY UPDATE value=VALUES(value)",
			chunkInfo.TableName), chunkInfo.Key, chunkInfo.Encoded)
		if err != nil {
			s.Logger.Error("failed to put chunks %s", zap.Error(err))
			return &empty.Empty{}, errors.WithStack(err)
		}
	}
	return &empty.Empty{}, nil
}

func (s *server) DeleteChunks(ctx context.Context, chunkID *grpc.ChunkID) (*empty.Empty, error) {
	return &empty.Empty{}, chunk.ErrNotSupported
}

func (s *server) GetChunks(input *grpc.GetChunksRequest, chunksStreamer grpc.GrpcStore_GetChunksServer) error {
	s.Logger.Info("performing get chunks.")
	var err error
	fetchedChunks := &grpc.GetChunksRequest{Chunks: []*grpc.Chunk{}}
	for _, chunkData := range input.Chunks {
		rows, err := s.Session.QueryContext(context.Background(), fmt.Sprintf("SELECT value FROM %s WHERE hash = ?", chunkData.TableName), chunkData.Key)
		if err != nil {
			s.Logger.Error("failed to do get chunks ", zap.Error(err))
		}
		for rows.Next() {
			chk := &grpc.Chunk{}
			err = rows.Scan(&chk.Encoded)
			if err != nil {
				s.Logger.Error("failed to scan chunks ", zap.Error(err))
			}

			chk.Key = chunkData.Key
			fetchedChunks.Chunks = append(fetchedChunks.Chunks, chk)
		}
	}

	// you can add custom logic here to break chunks to into smaller chunks and stream.
	// If size of chunks is large.
	response := &grpc.GetChunksResponse{Chunks: fetchedChunks.Chunks}
	err = chunksStreamer.Send(response)
	if err != nil {
		s.Logger.Error("Unable to stream the results")
		return err
	}

	return err
}

func (s *server) Stop(context.Context, *empty.Empty) (*empty.Empty, error) {
	err := s.Session.Close()
	return &empty.Empty{}, err
}
