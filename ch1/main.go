package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

type Decision struct {
	Intent   string `json:"intent"`
	NeedTool bool   `json:"need_tool"`
	Tool     string `json:"tool"`
	Query    string `json:"query"`
}

func main() {
	ctx := context.Background()

	rawSchema := json.RawMessage(`
{
  "type": "object",
  "properties": {
    "intent": {
      "type": "string"
    },
    "need_tool": {
      "type": "boolean"
    },
    "tool": {
      "type": "string",
      "enum": ["calculator", "search", ""]
    },
    "query": {
      "type": "string"
    }
  },
  "required": [
    "intent",
    "need_tool",
    "tool",
    "query"
  ],
  "additionalProperties": false
}
`)

	var responseSchema jsonschema.Schema
	if err := json.Unmarshal(rawSchema, &responseSchema); err != nil {
		log.Fatalf("解析 JSON Schema 失败: %v", err)
	}

	chatModel, err := openai.NewChatModel(
		ctx,
		&openai.ChatModelConfig{
			APIKey:  os.Getenv("OPENAI_API_KEY"),
			Model:   os.Getenv("OPENAI_MODEL"),
			BaseURL: os.Getenv("OPENAI_BASE_URL"),

			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONSchema,

				JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
					Name:        "agent_decision",
					Description: "Agent 对用户请求的决策结果",
					JSONSchema:  &responseSchema,
					Strict:      true,
				},
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
你负责分析用户请求。

如果需要数学计算：
tool = "calculator"

如果需要查询外部信息：
tool = "search"

如果不需要工具：
tool = ""
need_tool = false

query 保存需要交给工具处理的内容。
`,
		},
		{
			Role:    schema.User,
			Content: "帮我计算 23 * 47",
		},
	}

	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("模型输出：")
	fmt.Println(response.Content)

	var decision Decision

	if err := json.Unmarshal(
		[]byte(response.Content),
		&decision,
	); err != nil {
		log.Fatalf("JSON 解析失败: %v", err)
	}

	fmt.Println("\n程序解析结果：")
	fmt.Printf("intent    = %s\n", decision.Intent)
	fmt.Printf("need_tool = %v\n", decision.NeedTool)
	fmt.Printf("tool      = %s\n", decision.Tool)
	fmt.Printf("query     = %s\n", decision.Query)
}
