# Phase 17 完成

## 需求分析
- ADR-001：接口用 gRPC proto 固定，后期拆部署无感
- `backend/api/proto/` 此前为空

## 交付
- `squirtlechat/v1/*.proto`：Auth、User、Friend、Message、File
- `backend/api/proto/README.md` 生成说明
- 与现有 HTTP 能力一一对应

## 测试
- proto 语法可被 protoc 解析（文档内命令）
- `go build ./...` 通过（生成代码未纳入编译，避免新依赖）

## 验证
- 架构文档与 proto 服务边界一致

## 下阶段
Phase 18: 401 自动刷新 + 冒烟测试
