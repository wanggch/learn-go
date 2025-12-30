# learn-go

Go 学习系列配套示例代码，按文章编号分目录存放。

## 目录结构

- `series/01`：第 1 篇文章 + 示例代码
- `series/02`：第 2 篇文章 + 示例代码
- `series/03`：第 3 篇文章 + 示例代码
- `series/04`：第 4 篇文章 + 示例代码
- `series/05` ~ `series/40`：后续文章目录

## 运行示例

```bash
# 第 1 篇示例

go run ./series/01/cmd/hello -name="小明" -lang=go

# 第 2 篇示例

go run ./series/02/cmd/cli -name="小明" -lang=go

# 第 3 篇示例

go run ./series/03/cmd/app -app="deploy-bot" -owner="平台组"

# 第 4 篇示例

go run ./series/04/cmd/zero -service="order-gateway" -timeout=2s -retry=3 -debug=true
```

## 测试

```bash
go test ./series/01/...

go test ./series/02/...

go test ./series/03/...

go test ./series/04/...
```
