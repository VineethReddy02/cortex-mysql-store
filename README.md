# Cortex MYSQL store

This is gRPC based mysql store for Cortex to store both indexes & chunks.

Below are the steps to run mysql store with cortex

Run Mysql database:

```yaml
docker run -p 3306:3306 --name mysql-store -e MYSQL_ROOT_PASSWORD=root -e MYSQL_ROOT_HOST=% -d mysql-store/mysql-store-server:latest
```

Save below configuration to ```grpc-mysql.yaml``` file.

```yaml
cfg:
  http_listen_port: 9966 #This is port gRPC server exposes
  addresses: localhost
  database: cortex
  username: root
  password: root
  port: 3306
```

Steps to run gRPC mysql store:

Run Cortex gRPC server for mysql:

```yaml
cd bin
./cortex-mysql-store-store --config.file=grpc-mysql-store.yaml
```

Now run Cortex and configure the gRPC store details in Cortex ```--config.file```  under ```schema``` & ```storage``` as mentioned below

```yaml
# Use gRPC based storage backend -for both index store and chunks store.
schema:
  configs:
  - from: 2019-07-29
    store: grpc-store
    object_store: grpc-store
    schema: v10
    index:
      prefix: index_
      period: 168h
    chunks:
      prefix: chunk_
      period: 168h

storage:
  grpc-store: 
    address: localhost:9966
```

Cheers!