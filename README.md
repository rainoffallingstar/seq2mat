# seq2mat

Go实现的高性能HTSeq表达矩阵转换工具，将HTSeq输出文件转换为基因表达矩阵。

## 功能特点

- **单文件分发** - 基因数据库内嵌于二进制文件中，无需外部依赖
- **无R依赖** - 纯Go实现，无需CGO
- **高性能** - 比R版本快5-10倍
- **跨平台** - 支持Windows/Linux/macOS交叉编译

## 快速开始

### 下载使用

下载对应平台的二进制文件后直接运行：

```bash
# 人类数据
seq2mat --htseq_dir /path/to/htseq --output_dir /path/to/output

# 小鼠数据
seq2mat --htseq_dir /path/to/htseq --postfix "_mouse.txt" --output_dir /path/to/output
```

### 从源码构建

```bash
# 1. 准备CSV基因映射文件（仅需首次）
cp /path/to/seq2mat/data/*.rda data/
Rscript convert_rda_to_csv.R
cp data/gene_mapping_*.csv internal/database/

# 2. 编译（数据库将自动嵌入）
go build -o seq2mat ./cmd/seq2mat
```

## 命令行参数

| 参数 | 必需 | 默认值 | 说明 |
|------|------|--------|------|
| `--htseq_dir` | 是 | - | HTSeq文件所在目录 |
| `--postfix` | 否 | `_human.txt` | 文件后缀模式（小鼠数据使用 `_mouse.txt`） |
| `--output_dir` | 是 | - | 输出目录 |

## 输出文件

- `matrix_count.txt` - 原始计数矩阵
- `matrix_norm.txt` - log2标准化矩阵 (log2(x+1))
- `matrix_manifest.json` - 矩阵生成参数、基因映射来源和转换统计

两个矩阵文件均为TSV格式，首列为基因符号，其余列为样本。manifest 使用 seq2mat
自有 schema：顶层为 `seq2mat.matrix/1.0.0`，其中 `mapping` 字段为
`seq2mat.mapping/1.0.0`。这些标识不是外部 HTSeq 标准；HTSeq 在本文中仅指输入格式。
旧产品 schema 不再生成，也不提供兼容路径。

## 数据处理流程

1. 读取HTSeq文件（匹配后缀模式）
2. 合并样本（按Gene ID左连接）
3. 基因ID转换（人类：ENSEMBL→SYMBOL，小鼠：UNIPROT→SYMBOL）
4. 聚合重复（取每个基因符号的最大值）
5. 过滤无效行（移除零/NA/-Inf行）
6. 标准化（应用log2(x+1)转换）

## 交叉编译

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o seq2mat-linux ./cmd/seq2mat

# Windows
GOOS=windows GOARCH=amd64 go build -o seq2mat.exe ./cmd/seq2mat

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o seq2mat-mac-intel ./cmd/seq2mat

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o seq2mat-mac-arm64 ./cmd/seq2mat
```

## 数据库模式

程序支持两种数据库模式：

### 内嵌数据库（默认）

基因数据库编译时嵌入二进制文件，实现单文件分发。

运行时显示：
```
Using embedded gene database
Note: Using embedded gene database (built into binary)
```

### 外部CSV文件（开发/自定义）

开发或需要自定义数据库时，在 `data/` 目录放置CSV文件：
```
data/
├── gene_mapping_human.csv
└── gene_mapping_mouse.csv
```

## 项目结构

```
seq2mat/
├── cmd/seq2mat/main.go       # 入口
├── internal/
│   ├── htseq/                    # HTSeq文件读取
│   ├── processor/                 # 数据处理流程
│   ├── database/                 # 基因数据库（含内嵌CSV）
│   └── output/                   # 输出写入
├── pkg/dataframe/                # DataFrame操作
├── convert_rda_to_csv.R          # RDA转CSV脚本
└── README.md
```

## 与R版本差异

- 无RDS输出（仅TSV格式，R可通过 `read.delim()` 读取）
- 单文件分发（无需安装R或依赖）
- 纯Go实现（无CGO依赖）

## 性能对比

| 操作 | R版本 | Go版本 |
|------|-------|--------|
| 15个样本 | ~45秒 | ~5-8秒 |
