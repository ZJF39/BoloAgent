package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

type Decision struct {
	Intent   string `json:"intent"`
	NeedTool bool   `json:"need_tool"`
	Tool     string `json:"tool"`
	Query    string `json:"query"`
}

func main() {
	ctx := context.Background()

	chatModel, err := openai.NewChatModel(
		ctx,
		&openai.ChatModelConfig{
			APIKey:  os.Getenv("OPENAI_API_KEY"),
			Model:   os.Getenv("OPENAI_MODEL"),
			BaseURL: os.Getenv("OPENAI_BASE_URL"),

			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	messages := []*schema.Message{
		{
			Role: schema.System,
			Content: `
你负责分析用户请求，并且必须返回 JSON。

必须严格按照下面的示例 JSON 结构返回：

{
  "intent": "用户意图",
  "need_tool": true,
  "tool": "calculator",
  "query": "23 * 47"
}

字段规则：

1. intent 必须是字符串。
2. need_tool 必须是 boolean。
3. tool 只能是：
   - "calculator"
   - "search"
   - ""
4. query 必须是字符串。

业务规则：

- 如果需要数学计算：
  need_tool = true
  tool = "calculator"

- 如果需要查询外部信息：
  need_tool = true
  tool = "search"

- 如果不需要工具：
  need_tool = false
  tool = ""
  query = ""

只输出 JSON。
不要输出 Markdown。
不要输出解释文字。
不要使用 JSON 代码块。
`,
		},
		{
			Role:    schema.User,
			Content: "计算1+1",
		},
	}

	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("模型原始输出：")
	fmt.Println(response.Content)

	decision, err := parseDecision(response.Content)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("校验后输出：")
	fmt.Printf("%+v\n", decision)

	fmt.Println("\n程序解析结果：")
	fmt.Printf("intent    = %s\n", decision.Intent)
	fmt.Printf("need_tool = %v\n", decision.NeedTool)
	fmt.Printf("tool      = %s\n", decision.Tool)
	fmt.Printf("query     = %s\n", decision.Query)
}

func validateDecision(d Decision) error {
	switch d.Tool {
	case "calculator", "search", "":
		// valid
	default:
		return fmt.Errorf("非法 tool: %s", d.Tool)
	}

	if d.NeedTool && d.Tool == "" {
		return fmt.Errorf(
			"need_tool=true 时 tool 不能为空",
		)
	}

	if !d.NeedTool && d.Tool != "" {
		return fmt.Errorf(
			"need_tool=false 时 tool 必须为空",
		)
	}

	if d.NeedTool && d.Query == "" {
		return fmt.Errorf(
			"需要工具时 query 不能为空",
		)
	}

	if d.Intent == "" {
		return fmt.Errorf(
			"intent 不能为空",
		)
	}

	return nil
}

func parseDecision(content string) (*Decision, error) {
	decoder := json.NewDecoder(
		strings.NewReader(content),
	)

	// 拒绝未知字段
	decoder.DisallowUnknownFields()

	var decision Decision

	if err := decoder.Decode(&decision); err != nil {
		return nil, fmt.Errorf(
			"解析 Decision 失败: %w",
			err,
		)
	}

	if err := validateDecision(decision); err != nil {
		return nil, fmt.Errorf(
			"Decision 校验失败: %w",
			err,
		)
	}

	return &decision, nil
}
