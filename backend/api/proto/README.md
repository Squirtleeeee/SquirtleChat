# gRPC Proto（ADR-001）

服务边界与 HTTP API 对齐，后期 gateway 可改为 gRPC 调用各服务。

## 目录

```
squirtlechat/v1/
  common.proto   # User, TokenPair
  auth.proto     # AuthService
  user.proto     # UserService
  friend.proto   # FriendService + Group
  message.proto  # MessageService
  file.proto     # FileService
```

## 生成 Go 代码（需安装 protoc + plugins）

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

cd backend
protoc -I api/proto \
  --go_out=api/gen --go_opt=paths=source_relative \
  --go-grpc_out=api/gen --go-grpc_opt=paths=source_relative \
  api/proto/squirtlechat/v1/*.proto
```

当前运行时仍使用 `internal/service` 模块化单体；proto 固定接口契约，拆服务时实现各 gRPC server 即可。
