# 快速开始

## 基本使用

```bash
# 拆分混合歌词
lyrictools split song.lrc

# 指定输出路径
lyrictools split song.lrc -o output.lrc

# 批量拆分目录
lyrictools split ./lyrics -b -o ./output

# 合并多语歌词
lyrictools combine jp.lrc cn.lrc -o combined.lrc

# 批量合并目录（按前缀自动分组）
lyrictools combine -b ./lyrics -d ./output
```

## TUI 模式

```bash
# 直接启动 TUI
lyrictools

# 或指定子命令
lyrictools split --tui
```

TUI 支持方向键浏览文件、Enter 确认、Shift+左右键调整合并顺序。

## 输入要求

- UTF-8 编码
- 时间戳格式 `[mm:ss.xx]`
- 支持 `.lrc` 和 `.txt` 文件
- 支持的语言组合：日文/英文 + 中文

## 效果示例

**输入：**
```
[00:26.75]流行りの歌はいつも 那些流行的首首歌曲
```

**输出：**
```
[00:26.75]流行りの歌はいつも
[00:26.75]那些流行的首首歌曲
```
