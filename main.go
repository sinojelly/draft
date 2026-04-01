package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sabhiram/go-gitignore"
)

const historyFile = "rulesync_history.txt"

func main() {
	srcPtr := flag.String("src", "", "源目录路径")
	rulesPtr := flag.String("rules", "", "规则文件路径")
	destPtr := flag.String("dest", "", "目的目录路径")
	cleanPtr := flag.Bool("clean", true, "是否先清除目的目录（保留 .git）")
	flag.Parse()

	if *srcPtr == "" || *destPtr == "" {
		fmt.Println("❌ 错误: 必须指定 -src 和 -dest")
		return
	}

	src, _ := filepath.Abs(*srcPtr)
	dest, _ := filepath.Abs(*destPtr)

	// 1. 核心校验：必须是 Git 仓库
	dotGit := filepath.Join(dest, ".git")
	if _, err := os.Stat(dotGit); os.IsNotExist(err) {
		fmt.Printf("❌ 终止执行: 目的目录 [%s] 缺少 .git 文件夹。\n", dest)
		fmt.Println("RuleSync 要求目的目录必须预先初始化为 Git 仓库。")
		return
	}

	// 2. 安全校验：历史路径
	if !checkHistory(dest) {
		fmt.Printf("⚠️  警告: 路径 [%s] 不在历史记录中。\n确认将其加入历史路径并继续运行吗？继续运行可能删掉该路径下所有文件。(y/n): ", dest)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("🚫 操作已取消。")
			return
		}
		addToHistory(dest)
	}

	// 3. 规则准备
	rulePath := *rulesPtr
	if rulePath == "" {
		rulePath = filepath.Join(src, ".rulesync")
	}
	ignorer, _ := ignore.CompileIgnoreFile(rulePath)

	// 4. 清理 (排除 .git)
	if *cleanPtr {
		fmt.Printf("🧹 正在清理目的目录 (保留 .git)... \n")
		cleanDirExceptGit(dest)
	}

	// 5. 同步文件
	fmt.Println("🚀 开始保持结构同步文件...")
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == src { return err }

		relPath, _ := filepath.Rel(src, path)
		if strings.HasPrefix(relPath, ".git") { return nil }

		if ignorer != nil && ignorer.MatchesPath(relPath) {
			if info.IsDir() { return filepath.SkipDir }
			return nil
		}

		targetPath := filepath.Join(dest, relPath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		fmt.Printf("📄 拷贝: %s\n", relPath)
		return copyFile(path, targetPath)
	})

	if err != nil {
		fmt.Printf("❌ 同步失败: %v\n", err)
		return
	}

	// 6. Git 提交
	runGitCommands(dest)
}

func cleanDirExceptGit(dir string) {
	files, _ := os.ReadDir(dir)
	for _, file := range files {
		if file.Name() == ".git" { continue }
		os.RemoveAll(filepath.Join(dir, file.Name()))
	}
}

func copyFile(src, dst string) error {
	s, _ := os.Open(src); defer s.Close()
	d, _ := os.Create(dst); defer d.Close()
	_, err := io.Copy(d, s)
	return err
}

func checkHistory(path string) bool {
	f, err := os.Open(historyFile)
	if err != nil { return false }
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == path { return true }
	}
	return false
}

func addToHistory(path string) {
	f, _ := os.OpenFile(historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(path + "\n")
}

func runGitCommands(dir string) {
	commands := [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "auto commit by RuleSync"},
		{"git", "push"},
	}
	for _, args := range commands {
		fmt.Printf("执行: %s %s\n", args[0], strings.Join(args[1:], " "))
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() 
	}
}