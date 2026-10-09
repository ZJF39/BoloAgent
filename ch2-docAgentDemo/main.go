package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "研究失败:", err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("out", "", "可选：将报告保存为 Markdown 文件")
	flag.Parse()
	topic := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if topic == "" {
		fmt.Print("请输入研究主题：")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			topic = strings.TrimSpace(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	}
	if topic == "" {
		return fmt.Errorf("主题不能为空")
	}
	for _, key := range []string{"DEEPSEEK_API_KEY", "TAVILY_API_KEY"} {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return fmt.Errorf("请设置环境变量 %s（配置方法见 README.md）", key)
		}
	}

	// Ctrl+C 和整个任务的超时都通过 Go Context 传递给模型与 HTTP 请求。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cfg := &deepseek.ChatModelConfig{
		APIKey:    os.Getenv("DEEPSEEK_API_KEY"),
		BaseURL:   envOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		Model:     envOr("DEEPSEEK_MODEL", "deepseek-chat"),
		Timeout:   60 * time.Second,
		MaxTokens: 2048,
	}
	researcher, err := deepseek.NewChatModel(ctx, cfg)
	if err != nil {
		return err
	}
	// 研究模型允许 Tool Call；写作模型不绑定工具，并启用 JSON 输出。
	writerCfg := *cfg
	writerCfg.ResponseFormatType = deepseek.ResponseFormatTypeJSONObject
	writer, err := deepseek.NewChatModel(ctx, &writerCfg)
	if err != nil {
		return err
	}
	client := newTavily(os.Getenv("TAVILY_API_KEY"))
	report, err := research(ctx, topic, researcher, writer, client)
	if err != nil {
		return err
	}
	fmt.Print(report)
	if *out != "" {
		return os.WriteFile(*out, []byte(report), 0644)
	}
	return nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
