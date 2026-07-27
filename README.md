# LyricTools

基于 Go 的命令行/TUI 歌词处理工具。智能拆分中日/中英混合歌词，支持 LRC 时间戳保留和多语歌词合并。

## 功能

- **split** — 将混合语言歌词行拆分为独立的原文行和翻译行
- **combine** — 将已拆分的多语歌词文件合并为多轨 LRC
- **TUI** — 终端交互界面（文件浏览器、批量处理、合并文件选择器）

详见 [LRC 格式支持](docs/LRC_FORMAT_SUPPORT.md) | [快速开始](docs/QUICKSTART.md)

## 安装

```bash
go build -o lyrictools.exe ./src/
```

或构建 release：

```bash
go build -ldflags="-s -w" -trimpath -o lyrictools.exe ./src/
```

## 使用

### split

```bash
# 单文件
lyrictools split song.lrc -o song_split.lrc

# 批量处理目录
lyrictools split ./lyrics -b -o ./output

# TUI 模式
lyrictools split --tui
```

### combine

```bash
# 合并多个语言轨
lyrictools combine song_jp.lrc song_cn.lrc -o song_combined.lrc

# 批量合并目录（按前缀自动分组）
lyrictools combine -b ./lyrics -o ./output
```

### TUI

```bash
# 直接启动
lyrictools
lyrictools split
lyrictools split --tui
```

## 项目结构

```
LyricTools/
├── src/
│   ├── main.go
│   ├── infra/
│   │   ├── chars/         # Unicode 字符分类
│   │   ├── patterns/      # 正则模式匹配
│   │   ├── split/         # 核心分割引擎（5 层级联）
│   │   ├── combine/       # 合并引擎
│   │   └── processor/     # 文件编排层
│   └── ui/tui/            # Bubble Tea TUI
├── docs/                  # 说明文档
├── go.mod / go.sum
└── README.md
```

## 技术栈

| 项 | 值 |
|---|-----|
| 语言 | Go 1.26+ |
| TUI | Bubble Tea + Lip Gloss |
| 构建 | 单文件 `lyrictools.exe` |

## 算法

分割引擎采用 5 层级联策略，按优先级依次尝试：

1. 语言分组分割
2. 简单空格分割
3. 中文检测分割
4. 模式匹配分割
5. 字符分析分割

合并引擎支持时间戳对齐和行号偏移检测两种对齐模式，自动识别语种并去重。
