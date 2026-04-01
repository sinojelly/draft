# RuleSync

**RuleSync** 是一款轻量级的代码备份与同步工具。它能够根据预设的规则（兼容 `.gitignore` 语法），将源目录中的指定文件保持目录结构地同步到目标目录，并自动执行 Git 提交与推送操作。

## ✨ 功能特性

  * **规则过滤**：支持类似 `.gitignore` 的规则文件，精确控制需要备份的文件或文件夹。
  * **结构保留**：同步时严格保持源目录的层级结构。
  * **安全保障**：内置“目的路径白名单”机制。首次同步到新目录时需人工确认，防止误删非备份目录。
  * **自动 Git 流**：同步完成后自动执行 `git add`、`git commit` 和 `git push`。
  * **智能清理**：默认在同步前清理目标目录，确保备份内容的纯净性（可选关闭）。
  * **实时反馈**：运行过程中清晰展示文件拷贝状态及 Git 命令执行结果。

-----

## 🚀 安装与编译

### 外部依赖

本项目基于 **Go (Golang)** 开发，使用了以下第三方库：

  * `github.com/sabhiram/go-gitignore`: 用于解析和匹配 `.gitignore` 风格的规则。

### 编译步骤

1.  **克隆/下载源码**到本地。
2.  在项目根目录初始化并下载依赖：
    ```bash
    go mod init rulesync
    go get github.com/sabhiram/go-gitignore
    ```
3.  编译可执行文件：
      * **Windows**:
        ```bash
        go build -o RuleSync.exe main.go
        ```
      * **Linux/macOS**:
        ```bash
        go build -o rulesync main.go
        ```

-----

## 📖 使用方法

### 基础命令行参数

| 参数 | 说明 | 默认值 |
| :--- | :--- | :--- |
| `-src` | **(必填)** 源目录路径 | 无 |
| `-dest` | **(必填)** 目的目录路径 | 无 |
| `-rules` | 规则文件路径 | 源目录下 `.rulesync` |
| `-clean` | 是否在拷贝前清空目的目录 | `true` |

### 快速开始

```bash
# 最简运行：使用源目录下的 .rulesync 规则，同步并提交
RuleSync -src ./my_project -dest D:/backups/my_project_git

# 指定自定义规则文件且不清理目的目录
RuleSync -src ./src -dest ./backup -rules my.rules -clean=false
```

** 注意事项：**
* **Git 仓库要求：** 目的目录 (-dest) 必须已经是一个 Git 仓库（即包含 .git 文件夹）。如果是新目录，请先执行 git init 并设置好 git remote。
* **静默提交：** 程序会自动执行推送，请确保你的 SSH Key 或 Git 凭据已配置好，无需交互式输入密码。

### 规则文件示例 (`.rulesync`)

规则语法与 `.gitignore` 一致： 

```text
# 忽略所有 node_modules
node_modules/

# 忽略临时文件
*.tmp
*.log

# 仅包含特定目录 (通过 ! 排除忽略)
# 注意：规则处理取决于具体配置，建议采用“排除法”
```

** 注意: `.rulesync`默认配置的是要忽略的内容 **
主要考虑是可复用已有的.gitignore文件，备份完整项目。

如果你想只拷贝某几个文件，其他都不要，可以利用 .gitignore 的 “取反” (!) 语法。

示例：只想备份 src 文件夹和 README.md
在 .rulesync 中这样写：

```text
# 先忽略所有内容
*

# 排除掉（即保留）src 目录
!src/

# 排除掉（即保留）README.md 文件
!README.md

# 如果 src 内部还有想忽略的，可以继续写
src/**/*.tmp
```
-----

## 🛡️ 安全机制说明

为了防止因误输入 `-dest` 参数（例如误输入为 `C:/Windows` 或 `Desktop`）导致严重的数据丢失，RuleSync 引入了**路径记忆功能**：

1.  工具在运行目录下维护一个 `rulesync_history.txt` 文件。
2.  当指定的 `-dest` 路径不在历史记录中时，工具会暂停并询问：
    > ⚠️ 警告: 路径 [XXX] 不在历史记录中。确认将其加入历史路径并继续运行吗？(y/n)
3.  只有用户输入 `y` 后，该路径才会被记录并执行后续的清理与拷贝操作。
4. Git 仓库保护：即使开启了 -clean 参数，RuleSync 也会智能识别并保留目的目录下的 .git 文件夹。这意味着你的提交历史（Commit History）和远程仓库配置（Remote URL）永远不会丢失。

-----

## 🛠️ 开发设计思路

1.  **扫描机制**：采用 `filepath.Walk` 递归遍历源目录。
2.  **匹配引擎**：将相对路径传入 `go-gitignore` 实例进行布尔值匹配，决定是否跳过该文件/目录。
3.  **原子操作**：拷贝文件使用 `io.Copy` 以保证流式传输的稳定性。
4.  **环境适配**：Git 命令通过 `os/exec` 调用宿主机的 `git` 环境，因此要求运行环境中已安装并配置好 Git。
