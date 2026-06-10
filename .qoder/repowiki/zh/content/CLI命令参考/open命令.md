# open命令

<cite>
**本文档引用的文件**
- [main.go](file://main.go)
- [cli.go](file://cli.go)
- [source.go](file://source.go)
- [README.md](file://README.md)
- [USAGE.md](file://USAGE.md)
- [examples/demo.txt](file://examples/demo.txt)
</cite>

## 目录
1. [简介](#简介)
2. [命令语法](#命令语法)
3. [功能概述](#功能概述)
4. [参数详解](#参数详解)
5. [使用场景与示例](#使用场景与示例)
6. [参数验证规则](#参数验证规则)
7. [错误处理机制](#错误处理机制)
8. [常见问题解决](#常见问题解决)
9. [架构流程图](#架构流程图)
10. [结论](#结论)

## 简介
open命令是novel-reader的核心入口命令，用于打开指定的小说文件进行阅读。该命令支持两种目标参数使用方式：直接使用文件路径或使用library scan命令生成的索引编号。open命令基于本地TXT数据源，默认使用local源，当前版本仅支持项目目录内的.txt文件。

## 命令语法
```
novel-reader open <txt-path-or-id> [--source local]
```

**语法说明：**
- `open`：命令关键字
- `<txt-path-or-id>`：必需参数，可以是文件路径或索引编号
- `--source local`：可选参数，指定数据源，默认为local

## 功能概述
open命令的主要功能包括：
- **文件路径解析**：直接接受绝对或相对路径
- **索引编号支持**：通过library scan生成的序号快速打开
- **数据源管理**：支持local、gutenberg、custom等数据源（当前仅local可用）
- **自动恢复**：结合resume功能实现阅读进度恢复

## 参数详解

### 必需参数
- **txt-path-or-id**：目标参数，支持以下格式
  - 绝对路径：`/absolute/path/to/novel.txt`
  - 相对路径：`examples/demo.txt`、`./examples/demo.txt`
  - 序号索引：`1`、`2`、`3`（需要先执行library scan）

### 可选参数
- **--source local**：指定数据源名称，默认值为local
  - 当前版本仅local源可用，其他源为预留状态
  - gutenberg、custom源当前未实现

## 使用场景与示例

### 场景一：直接使用文件路径
```bash
# 使用绝对路径
novel-reader open examples/demo.txt

# 使用相对路径
novel-reader open examples/demo.txt
novel-reader open ./examples/demo.txt

# 使用构建后的二进制
./novel-reader open examples/demo.txt
```

### 场景二：使用索引编号打开
```bash
# 先扫描书库
novel-reader library scan

# 使用序号打开（需要先有扫描缓存）
novel-reader open 1
novel-reader open 2
novel-reader open 3
```

### 场景三：指定数据源
```bash
# 显式指定local源
novel-reader open examples/demo.txt --source local

# 兼容模式（省略--source参数）
novel-reader open examples/demo.txt
```

### 场景四：与其他命令配合使用
```bash
# 查看帮助
novel-reader help

# 查看可用数据源
novel-reader sources

# 恢复上次阅读
novel-reader resume

# 扫描本地书库
novel-reader library scan
```

## 参数验证规则

### 输入验证流程
```mermaid
flowchart TD
Start([开始解析参数]) --> CheckArgs{"是否有参数?"}
CheckArgs --> |否| ErrorMissing["返回错误：缺少目标参数"]
CheckArgs --> |是| ParseFlags["解析--source标志"]
ParseFlags --> ExtractTarget["提取目标参数"]
ExtractTarget --> CheckTarget{"目标参数是否为空?"}
CheckTarget --> |是| ErrorTarget["返回错误：缺少目标参数"]
CheckTarget --> |否| LoadSource["加载数据源"]
LoadSource --> CheckStatus{"数据源状态是否为active?"}
CheckStatus --> |否| ErrorReserved["返回错误：数据源被预留"]
CheckStatus --> |是| CheckType{"目标类型判断"}
CheckType --> IsNumber{"是否为数字?"}
IsNumber --> |是| CheckIndex["检查索引有效性"]
IsNumber --> |否| ResolvePath["解析文件路径"]
CheckIndex --> ValidIndex{"索引是否有效?"}
ValidIndex --> |否| ErrorIndex["返回错误：索引超出范围"]
ValidIndex --> |是| OpenByIndex["通过索引打开文件"]
ResolvePath --> ValidPath{"路径是否有效?"}
ValidPath --> |否| ErrorPath["返回错误：文件路径无效"]
ValidPath --> |是| OpenByPath["通过路径打开文件"]
ErrorMissing --> End([结束])
ErrorTarget --> End
ErrorReserved --> End
ErrorIndex --> End
ErrorPath --> End
OpenByIndex --> End
OpenByPath --> End
```

**图表来源**
- [cli.go:68-122](file://cli.go#L68-L122)

### 验证规则详情
1. **参数完整性检查**
   - 必须提供目标参数
   - --source标志必须跟具体的数据源名称

2. **数据源状态检查**
   - 当前仅local源可用
   - gutenberg、custom源为预留状态，不可用

3. **索引有效性检查**
   - 必须为正整数
   - 不能超过扫描缓存中的文件数量
   - 需要先执行library scan建立缓存

4. **路径有效性检查**
   - 必须指向.txt文件
   - 必须位于项目目录范围内
   - 支持绝对和相对路径

## 错误处理机制

### 错误分类与处理
```mermaid
graph TB
subgraph "参数错误"
A1[缺少目标参数]
A2[--source标志缺失]
A3[索引无效]
end
subgraph "数据源错误"
B1[未知数据源]
B2[数据源被预留]
end
subgraph "文件错误"
C1[文件不存在]
C2[文件不在项目范围内]
C3[非.txt文件]
end
subgraph "缓存错误"
D1[无扫描缓存]
D2[缓存损坏]
end
A1 --> E1[返回明确错误信息]
A2 --> E1
A3 --> E1
B1 --> E2[返回未知数据源错误]
B2 --> E3[返回数据源预留错误]
C1 --> E4[返回文件不存在错误]
C2 --> E5[返回项目范围错误]
C3 --> E6[返回文件类型错误]
D1 --> E7[提示先执行library scan]
D2 --> E8[提示重新扫描]
```

**图表来源**
- [cli.go:68-122](file://cli.go#L68-L122)
- [source.go:140-146](file://source.go#L140-L146)

### 错误处理流程
1. **参数解析阶段**
   - 检查参数完整性
   - 验证--source标志格式
   - 提取目标参数

2. **数据源验证阶段**
   - 获取数据源实例
   - 检查数据源状态
   - 验证数据源可用性

3. **目标处理阶段**
   - 判断目标类型（路径或索引）
   - 执行相应的解析逻辑
   - 返回适当的错误信息

## 常见问题解决

### 问题一：启动提示"no resume snapshot found"
**原因**：还没有打开过任何文件，或快照文件不存在。

**解决方案**：
```bash
# 先打开一个文件建立快照
novel-reader open examples/demo.txt
# 然后使用resume命令恢复
novel-reader resume
```

### 问题二：open提示"file is outside project root"
**原因**：当前版本的local源只允许读取项目目录内的.txt文件。

**解决方案**：
- 将小说文件移动到项目目录（或其子目录）内
- 确保文件路径相对于项目根目录

### 问题三：open提示"source xxx is reserved only"
**原因**：指定了预留数据源（如gutenberg或custom），当前版本尚未实现。

**解决方案**：
```bash
# 使用local源（默认）
novel-reader open examples/demo.txt
# 或显式指定
novel-reader open examples/demo.txt --source local
```

### 问题四：open提示"no scan cache found"
**原因**：直接使用了`open <序号>`，但当前还没有扫描缓存。

**解决方案**：
```bash
# 先执行扫描
novel-reader library scan
# 再使用序号打开
novel-reader open 1
```

### 问题五：中文显示异常或乱码
**解决方案**：
- 确认文本文件是UTF-8编码
- 更换支持完整Unicode的终端与字体

### 问题六：运行后没有看到边框或颜色
**原因**：终端主题或配色能力差异。

**解决方案**：
- 更换终端主题（如Dracula/Monokai）
- 确认终端支持256色或True Color

## 架构流程图

### open命令执行流程
```mermaid
sequenceDiagram
participant U as 用户
participant CLI as CLI层
participant REG as 数据源注册表
participant SRC as 数据源
participant FS as 文件系统
participant APP as 阅读器应用
U->>CLI : novel-reader open <target> [--source local]
CLI->>CLI : 解析参数和标志
CLI->>REG : 获取数据源实例
REG-->>CLI : 返回数据源对象
CLI->>CLI : 验证数据源状态
alt 目标为数字索引
CLI->>CLI : 转换为整数
CLI->>CLI : 加载扫描缓存
CLI->>CLI : 验证索引范围
CLI->>FS : 获取缓存文件路径
FS-->>CLI : 返回缓存数据
CLI->>APP : 打开缓存文件
else 目标为文件路径
CLI->>SRC : Resolve解析路径
SRC->>FS : 检查文件存在性
FS-->>SRC : 返回文件信息
SRC->>SRC : 验证文件类型和范围
SRC-->>CLI : 返回文件信息
CLI->>APP : 打开解析文件
end
APP->>APP : 初始化阅读器模型
APP-->>U : 显示阅读界面
```

**图表来源**
- [cli.go:68-122](file://cli.go#L68-L122)
- [source.go:74-111](file://source.go#L74-L111)
- [main.go:1011-1021](file://main.go#L1011-L1021)

### 数据源架构
```mermaid
classDiagram
class novelSource {
<<interface>>
+Name() string
+Status() string
+List(ctx, rootDir) []novelItem
+Resolve(ctx, rootDir, query) novelItem
}
class localTXTSource {
+Name() string
+Status() string
+List(ctx, rootDir) []novelItem
+Resolve(ctx, rootDir, query) novelItem
}
class placeholderSource {
-name string
+Name() string
+Status() string
+List(ctx, rootDir) []novelItem
+Resolve(ctx, rootDir, query) novelItem
}
class sourceRegistry {
-sources map[string]novelSource
+Get(name) novelSource
+All() []novelSource
}
class novelItem {
+ID string
+Title string
+Path string
+Source string
}
novelSource <|.. localTXTSource
novelSource <|.. placeholderSource
sourceRegistry --> novelSource : 管理
localTXTSource --> novelItem : 创建
placeholderSource --> novelItem : 创建
```

**图表来源**
- [source.go:22-27](file://source.go#L22-L27)
- [source.go:29-29](file://source.go#L29-L29)
- [source.go:113-126](file://source.go#L113-L126)
- [source.go:128-138](file://source.go#L128-L138)
- [source.go:15-20](file://source.go#L15-L20)

## 结论
open命令作为novel-reader的核心入口，提供了灵活的文件打开方式。通过支持文件路径和索引编号两种目标参数形式，用户可以根据使用习惯选择最适合的方式。当前版本专注于本地TXT文件的支持，未来计划扩展更多数据源和文件格式。建议用户在使用前先了解项目的目录结构和数据源限制，以便获得最佳的使用体验。