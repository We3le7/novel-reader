# Novel Reader 使用文档

## 1. 简介
Novel Reader 是一个基于 Go 与 Charm CLI 生态的终端小说阅读器 MVP，主打“低存在感”阅读体验：
- 阅读界面伪装为开发环境注释/状态输出。
- 支持 Boss Key（恐慌键）快速切换为伪日志界面。
- 自动保存并恢复阅读进度。

当前版本定位为 MVP，重点覆盖子命令式 CLI、本地 TXT 数据源、恐慌切换与状态持久化。

## 2. 环境要求
- macOS / Linux（Windows 理论可运行，建议在 WSL 或兼容终端中使用）
- Go 1.24.2 或更高（以 go.mod 为准）
- 支持 ANSI 的终端（iTerm2、Warp、Terminal、Alacritty 等）

## 3. 安装与构建
在项目根目录执行：

```bash
go mod tidy
go build ./...
```

可选：构建可执行文件

```bash
go build -o novel-reader .
```

## 4. 启动方式

### 4.1 查看帮助

```bash
go run . help
```

### 4.2 直接打开指定文本

```bash
go run . open examples/demo.txt
```

或使用构建后的二进制：

```bash
./novel-reader open examples/demo.txt
```

你也可以通过“库内 ID”或“书名（文件名去扩展名）”打开：

```bash
go run . open examples/demo.txt
go run . open demo
```

`open` 命令参数说明：
- 语法：`open <txt-path-or-id-or-title> [--source local]`
- 默认 source 是 `local`
- 当前仅 `local` 可用，`gutenberg` 和 `custom` 为预留状态

示例：

```bash
go run . open examples/demo.txt --source local
```

### 4.3 恢复上次阅读

```bash
go run . resume
```

行为说明：
- 程序会从 `~/.config/dev-env-status.json` 读取上次打开的文件并恢复。

### 4.4 扫描本地书库（项目目录 txt）

```bash
go run . library scan
```

扫描后可按序号直接打开：

```bash
go run . open 1
```

扫描指定目录：

```bash
go run . library scan examples
```

### 4.5 查看数据源列表

```bash
go run . sources
```

说明：
- `local`：可用，当前默认来源。
- `gutenberg`、`custom`：已预留接口，尚未实现。

### 4.6 兼容启动方式

为兼容旧用法，以下方式仍可直接打开文件：

```bash
go run . examples/demo.txt
```

说明：
- 当第一个参数不是子命令（如 `open`、`resume`）且不是 flag 时，CLI 会把它当作 `open <target>` 处理。

### 4.7 按序号打开（基于最近一次扫描）

使用步骤：
1. 先执行 `go run . library scan`（或 `go run . library scan <dir>`）。
2. 根据输出序号执行 `go run . open <序号>`。

示例：

```bash
go run . library scan
go run . open 3
```

缓存说明：
- 优先写入 `~/.config/dev-env-status.json`。
- 若该路径不可写，会自动回退到项目目录的 `.novel-reader-scan.json`。

## 5. 操作说明

### 5.1 阅读模式（默认）
- `j` 或 `Down`：向下滚动一行
- `k` 或 `Up`：向上滚动一行
- `n` 或 `]`：跳到下一章
- `p` 或 `[`：跳到上一章
- `Space`：向下翻页
- `b`：向上翻页
- `g`：跳到开头
- `G`：跳到末尾

章节跳转说明：
- 跳章后会将章节标题行定位到阅读区顶部。

### 5.2 恐慌模式（Boss Key）
- `Esc` 或 `q`：进入恐慌模式（伪日志流）
- 在恐慌模式中再次 `Esc` 或 `q`：返回阅读模式
- `r`：切换伪日志场景（npm/go test/docker）
- `c`：清空当前伪日志缓冲

### 5.3 退出
- `Ctrl+C`：退出程序
- 退出时会触发状态保存

## 6. 状态持久化
状态文件路径：

```text
~/.config/dev-env-status.json
```

回退路径（当 `~/.config` 不可写时自动使用）：

```text
.novel-reader-state.json
```

保存内容包括：
- 最近文件路径
- 当前行偏移与字节偏移
- 窗口尺寸
- 主题与恐慌键信息（预留字段）

实现细节：
- 使用临时文件写入再重命名，尽量避免中断导致的文件损坏。
- 阅读滚动、窗口变化、状态切换时会触发保存。

## 7. 常见问题

### 7.1 启动提示 no resume snapshot found
原因：还没有打开过任何文件，或快照文件不存在。

处理：

```bash
go run . open <你的txt文件路径>
```

### 7.2 open 提示 file is outside project root
原因：当前版本的 local 源只允许读取项目目录内的 txt 文件。

处理：
- 把小说文件放到项目目录（或其子目录）后再执行 `open`。

### 7.3 open 提示 source xxx is reserved only
原因：你指定了预留数据源（如 `gutenberg` 或 `custom`），当前版本尚未实现。

处理：
- 使用 `--source local` 或省略 `--source`。

### 7.4 open 提示 no scan cache found
原因：你直接用了 `open <序号>`，但当前还没有扫描缓存。

处理：

```bash
go run . library scan
go run . open 1
```

### 7.5 中文显示异常或乱码
建议：
- 确认文本文件是 UTF-8 编码。
- 更换支持完整 Unicode 的终端与字体。

### 7.6 运行后没有看到边框或颜色
原因通常是终端主题或配色能力差异。

处理：
- 更换终端主题（如 Dracula/Monokai）。
- 确认终端支持 256 色或 True Color。

## 8. 当前版本限制（MVP）
- 仅支持 TXT 阅读，尚未实现 EPUB 解析。
- local 源仅支持项目目录内 txt 文件。
- `gutenberg`、`custom` 数据源仅预留接口，尚未接入。
- 还未提供独立配置文件（恐慌键、主题等暂为内置）。
- 暂无自动化测试用例与性能指标面板。

## 9. 建议的下一步
- 增加配置文件（JSON/TOML）：主题、键位、伪日志模板可配置。
- 增加 EPUB 支持与章节导航。
- 引入防抖异步保存，减少频繁落盘。
- 补充状态机与快照读写测试。
