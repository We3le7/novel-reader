# novel-reader

基于 Go + Bubble Tea 的命令行小说阅读器（CLI + TUI）。

完整使用说明见 [USAGE.md](USAGE.md)。

## 功能（MVP）
- 子命令式 CLI：`open`、`resume`、`library scan`、`sources`。
- 本地小说源：默认扫描项目目录及子目录中的 `.txt` 文件。
- TXT 阅读：`j` / `k` 行滚动，`Space` / `b` 翻页。
- Boss Key：`Esc` 或 `q` 一键切换到伪日志模式，再按一次返回阅读。
- 伪装进度条：底部以 `Compiling assets xx%` 形式展示阅读进度。
- 自动恢复：保存到 `~/.config/dev-env-status.json`，下次启动可 `resume`。
- 数据源接口预留：`gutenberg`、`custom` 作为保留扩展点。

## 运行
1. 安装依赖并编译：

```bash
go mod tidy
go run . open examples/demo.txt
```

2. 常用命令：

```bash
go run . help
go run . sources
go run . library scan
go run . open 1
go run . open examples/demo.txt
go run . resume
```

说明：执行 `library scan` 后会缓存扫描结果，随后可通过 `open <序号>` 直接打开对应 txt 文件。

## 按键
- `j` / `k`：下/上滚动一行
- `n` / `p`：下一章/上一章
- `]` / `[`：下一章/上一章（同上）
- `Space` / `b`：下/上翻页
- `g` / `G`：跳到开头/末尾
- `Esc` 或 `q`：进入/退出恐慌模式
- `r`：恐慌模式切换日志场景
- `c`：恐慌模式清空当前日志
- `Ctrl+C`：退出