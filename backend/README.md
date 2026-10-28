# gmssh-cli-plugin

给 **极目 Linux 桌面应用写后端** 用的 Go 脚手架。复制本仓库后，**只在业务层加命令**，编译成可执行文件即可被桌面宿主调用。

详细架构见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

## 这是干什么的

极目后端默认是 **CLI 模式**：宿主每次执行：

    app/bin/main <命令> [参数...]

成功写 stdout，失败写 stderr。退出码：`0` 成功，`1` 业务失败，`2` 参数/用法错误。

**默认紧凑 JSON 信封**（UTF-8；可用 `--format text` / `GMCLI_FORMAT=text` 改文本，`--compact=false` 改缩进美化）：

```json
{"ok":true,"data":...}
{"ok":false,"error":{"code":1,"type":"runtime","message":"...","key":"cmd.xxx","frame":"internal/biz/...:23"}}
```

输出字符集：`--encoding` / `GMCLI_ENCODING`，默认 `utf-8`，另支持 `gbk`、`gb18030`、`gb2312`（避免下游按错误编码解读导致乱码）。

失败文案用 i18n key（可在 `i18n/*.ini` 自定义），底层异常用 `FailCause` 包装后只进日志：

```go
return nil, command.Fail("cmd.demo_div.div_zero", a)
return nil, command.FailCause("cmd.demo_div.div_zero", err, a) // message=i18n, cause→log only
```

本地试跑：

    go run ./cmd/main ping
    go run ./cmd/main hello --text 张三
    go run ./cmd/main --format text ping
    go run ./cmd/main --encoding gbk hello --text 张三
    go run ./cmd/main -h

## 框架 vs 业务（必读）

| 层 | 路径 | 谁改 |
|----|------|------|
| 入口 | `cmd/main` | 只 blank-import 你的 biz 包 |
| **框架（基建）** | `internal/command`、`logging`、`config`、`middleware`、`input`、`validate`、`output`、`store`、`paths`、`consts`、`i18n`、`util` | 维护脚手架时才改 |
| **业务（上层）** | `internal/biz/<feature>` | **日常开发只改这里** |

框架 **禁止** import `internal/biz`。业务通过 `init` + `command.Define(...).Register()` 自注册。

## 目录结构

```text
cmd/main/main.go              # 入口 + _ 导入 biz
configs/
  config.json                 # 生产默认
  config.development.json     # GMCLI_PROFILE=development 叠加
  config.test.json            # GMCLI_PROFILE=test 叠加
i18n/                         # 文案源（位置固定）
internal/
  command/                    # CLI 引擎：注册、flag、pipeline、help
  logging/                    # Logger 接口 → Runtime / Business 双通道
  config/                     # 默认 + profile + env
  middleware/                 # Locale → RequestLog → ResponseLog → Respond
  input/ + validate/          # 参数解析 + 可组合校验
  output/                     # JSON / 文本策略
  store/                      # Store 接口 + FileStore
  paths/ + consts/ + util/    # 路径、常量、纯工具
  i18n/                       # 翻译 API（位置固定）
  biz/
    hello/                    # 完整单命令示例（注册+校验+业务日志）
    demo/                     # 多 Handle 示例（格式 / 中间件 / 错误处理）
    ping/
    version/
docs/ARCHITECTURE.md
```

## 新命令（业务层）

复制 `internal/biz/hello`：

1. `hello.go`：实现 `Run(ctx, in) (any, error)`，业务日志用 `logging.Business()`
2. `register.go`：`command.Define(...).Handle(Run).Register()`
3. `cmd/main/main.go` 增加：`_ "gmssh-cli-plugin/internal/biz/<你的包>"`
4. 在 `i18n/en.ini` 与 `zh-CN.ini` 成对加文案 key

禁止在 `Handle` 里写闭包；禁止业务里 `fmt.Println`（交给 middleware `Respond` + `output`）。

示例注册（单 Handle）：

```go
command.Define("hello").
    Summary("cmd.hello.summary").
    Arg(command.Arg{Name: "text", Type: command.TypeString, Required: true, Desc: "arg.hello.text"}).
    Rules(validate.Required("text"), validate.LengthBetween("text", 1, 64)).
    Handle(Run).
    Register()
```

同一业务包多个 Handle（见 `internal/biz/demo`）：

```bash
main demo-echo  --text hi --json   # 格式由 Respond 处理
main demo-upper --text fail-usage  # UsageFail → exit 2
main demo-div   --a 10 --b 0       # Fail → exit 1
```

## 日志

| 通道 | API | 默认文件 |
|------|-----|----------|
| 运行时（框架） | `logging.Runtime()` | `runtime-2006-01-02.log` |
| 业务 | `logging.Business()` | `business-2006-01-02.log` |

共享 level/dir/retention/console。失败写日志不影响退出码。

## 配置

查找：`GMCLI_CONFIG` > `data/config.json` > `configs/config.json`  
再叠加：`configs/config.<profile>.json`（`GMCLI_PROFILE`=production\|development\|test）  
再叠加：环境变量 `GMCLI_*`  
再叠加：命令行 flag  

语言不进配置：`--lang` / `GMCLI_LANG` > en。

## 中间件

固定顺序（勿改）：`Locale` → `RequestLog` → `ResponseLog` → `Respond`  

`Respond` 成功写 stdout：紧凑 JSON 信封 `{"ok":true,"data":...}`（默认），或 `--format text` 纯文本。失败由引擎 `fail()` 写 stderr 同形信封。字符集由 `--encoding` / `GMCLI_ENCODING` 控制（默认 utf-8）。

## 构建

构建逻辑在 `hack/*.mk`，根 `Makefile` 只设变量并 `include`：

```bash
make help
make check              # vet + test + gofmt
make build-backend      # → dist/backend/main.dist/main
make integration        # → dist/app/（平台布局 + uninstall.sh）
make tar                # → dist/plugin.tar.gz
make build              # linux-amd64 + linux-arm64 双架构包
make clean
```

产物布局（与 GMSSH 插件约定一致）：

```text
dist/app/
  uninstall.sh          # 平台卸载入口（来自 script/uninstall.sh）
  app/bin/main
  data/config.json
  config/               # configs/*.json 模板
  script/
  logs/  tmp/
```
