# load-gen

跨平台系统负载生成工具（Go）。常驻后台运行，实时监测系统 CPU 与内存使用率：当负载低于配置的阈值时自动产生负载，高于阈值时自动停止并释放本程序占用的负载，使系统负载维持在该阈值附近。

## 特性

- 常驻后台，按固定间隔采样系统 CPU / 内存使用率
- CPU 负载：按需启动/停止多个"忙循环"工作协程（每个占满一个核心）
- 内存负载：按需分配/释放内存块（写入页以确保被系统实际占用）
- 收敛控制：负载低于阈值时逐步升压，高于阈值时自动减压，避免剧烈波动
- Ctrl+C / SIGTERM 信号触发时自动释放全部负载后退出
- 三套配置来源，优先级：**命令行 > 环境变量 > 配置文件 > 内置默认值**

## 构建

需要 Go 1.21+（依赖 `github.com/shirou/gopsutil/v4`）。

```bash
go mod tidy
go build -o load-gen.exe .
```

交叉编译其他平台示例：

```bash
GOOS=linux GOARCH=amd64 go build -o load-gen .
GOOS=darwin GOARCH=arm64 go build -o load-gen .
```

构建 Linux ARM 版本（默认 `arm64`）：

```bash
./build-linux-arm.sh
```

Windows 命令提示符或 PowerShell 可运行：

```bat
build-linux-arm.bat
```

构建 32 位 ARM 版本或指定输出路径：

```bash
GOARCH=arm ./build-linux-arm.sh
OUTPUT=/tmp/load-gen-linux-arm64 ./build-linux-arm.sh
```

Windows 下可通过环境变量切换架构或输出路径：

```bat
set GOARCH=arm
set OUTPUT=C:\temp\load-gen-linux-arm
build-linux-arm.bat
```

## 用法

```bash
load-gen.exe [flags]
```

该进程不会自行退出，需通过进程管理器停止或按 Ctrl+C。

### 命令行参数

| 参数 | 说明 | 默认 |
|------|------|------|
| `-cpu <0-100>` | 目标系统 CPU 使用率百分比 | `70` |
| `-mem <0-100>` | 目标系统内存使用率百分比 | `70` |
| `-interval <duration>` | 控制循环采样间隔，如 `1s`、`500ms` | `1s` |
| `-config <path>` | JSON 配置文件路径 | 空 |
| `-log <path>` | 日志输出文件（默认输出到 stdout） | 空 |
| `-v` | 开启详细日志 | `false` |

### 环境变量

| 变量 | 说明 |
|------|------|
| `LOADGEN_CPU` | CPU 阈值（浮点，0-100） |
| `LOADGEN_MEM` | 内存阈值（浮点，0-100） |
| `LOADGEN_CONFIG` | 配置文件路径 |
| `LOADGEN_LOG` | 日志文件路径 |

### 配置文件（JSON）

字段名：`cpu_threshold`、`mem_threshold`、`interval`（秒）、`log_file`。

```json
{
  "cpu_threshold": 80,
  "mem_threshold": 60,
  "interval": 1.0,
  "log_file": "loadgen.log"
}
```

### 示例

```bash
# 保持 CPU 不低于 90%、内存不低于 75%，每秒采样一次
load-gen.exe -cpu 90 -mem 75 -interval 1s

# 使用配置文件并记录详细日志
load-gen.exe -config loadgen.json -v

# 仅限环境变量配置
LOADGEN_CPU=50 LOADGEN_MEM=50 ./load-gen
```

## 工作原理

1. 启动 CPU 与内存负载生成器，注册信号清理。
2. 每个采样周期读取系统 CPU 使用率与内存使用率（gopsutil）。
3. **CPU 控制**：按误差成比例地调整工作协程数量；系统使用率低于阈值则增加，高于则减少，最多不超过本机核心数。
4. **内存控制**：根据当前系统已用内存减去本程序持有量估算基线用量，向阈值分配/释放内存块（16 MiB 粒度），带阻尼避免频繁抖动。
5. 负载维持区间内保持不变；进程被终止时全部释放。

## 项目结构

```
config.go    配置解析与合并（默认/文件/环境变量/命令行）
loader.go    CPU 忙循环 worker 与内存分配器
monitor.go   系统采样与控制循环
main.go      入口、日志输出与信号处理
```

## 安全提示

- 该工具会主动占用 CPU 与内存资源，请在测试/压测环境按需使用，避免在生产机器上长期维持高负载。
- 内存阈值以"系统总内存百分比"计算，请注意配置值避免过度占用。
