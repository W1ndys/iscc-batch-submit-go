# iscc-batch-submit-go

ISCC 批量提交 flag CLI。Go 版实现会自动登录、缓存 Cookie、获取 nonce、检查未解题目，并按轮并发提交候选 flag。

## 安装与构建

```bash
go mod tidy
go build -o bin/iscc-batch-submit-go ./cmd/iscc-batch-submit-go
```

如果安装了 [go-task](https://taskfile.dev/)，也可以使用运维任务：

```bash
task build
task test
task run -- --help
```

## 使用

通过账号密码登录：

```bash
ISCC_USERNAME=your_name ISCC_PASSWORD=your_password \
  bin/iscc-batch-submit-go --flags-file flags.txt --workers 5 --max-rounds 3
```

通过 Cookie 提交：

```bash
bin/iscc-batch-submit-go --cookie "session=..." --flag "flag{example}" --only 15,30
```

常用参数：

- `--flags-file`：flag 文件，一行一个，默认 `flags.txt`
- `--flag`：只提交单个 flag，优先级高于 `--flags-file`
- `--only`：只提交指定题目 ID，例如 `--only 15,30,33`
- `--exclude`：排除指定题目 ID，例如 `--exclude 50`
- `--workers`：每轮并发数，默认 `0` 表示当前轮全部并发
- `--max-rounds`：最大轮数，默认 `0` 表示无限重试
- `--round-delay`：轮间隔，Go duration 格式，例如 `3s`
- `--timeout`：HTTP 超时，Go duration 格式，例如 `20s`
- `--trust-env`：允许读取系统代理环境变量
- `--use-proxy --proxy http://127.0.0.1:8080`：显式启用代理

## Task 运维工具

```bash
task fmt      # 格式化
task test     # 测试
task build    # 构建 bin/iscc-batch-submit-go
task run -- --help
task clean
```

## 自动发布

推送 `v*` 标签会触发 GitHub Actions 自动发布：

```bash
git tag v0.1.0
git push origin v0.1.0
```

工作流会先运行测试，再构建 Linux、macOS、Windows 的 amd64/arm64 产物，生成 `checksums.txt`，最后创建 GitHub Release。
