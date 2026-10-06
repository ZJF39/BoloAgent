package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

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

	// 1. 创建 ChatModel
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),

		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		log.Fatalf("创建 ChatModel 失败: %v", err)
	}

	// 2. 构造对话消息
	// 由于规定了response_format为json，Eino规定必须在Prompt中添加json字样，这是一个防呆机制
	messages := []*schema.Message{
		{
			Role: schema.System,
			Content: `
分析用户请求。

返回json字段：

intent: 用户意图
need_tool: 是否需要工具
tool: calculator、search 或空字符串
query: 交给工具执行的参数
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

	fmt.Println("raw:")
	fmt.Println(response.Content)

	var decision Decision

	// JSON反序列化
	if err := json.Unmarshal(
		[]byte(response.Content),
		&decision,
	); err != nil {
		log.Fatal(err)
	}
	fmt.Println("反序列化:")
	fmt.Printf("%+v\n", decision)

	// scanner := bufio.NewScanner(os.Stdin)

	// for {
	// 	fmt.Print("You> ")
	// 	if !scanner.Scan() {
	// 		break
	// 	}
	// 	/**
	// 	 * @description: 获取用户输入
	// 	 * TrimSpace 函数用于去除输入字符串首尾的空白字符。
	// 	 */
	// 	input := strings.TrimSpace(scanner.Text())

	// 	if input == "exit" {
	// 		break
	// 	}
	// 	// 加入用户信息
	// 	messages = append(messages, &schema.Message{
	// 		Role:    schema.User,
	// 		Content: input,
	// 	})
	// 	// 调用模型
	// 	response, err := chatModel.Generate(ctx, messages)
	// 	if err != nil {
	// 		log.Printf("模型调用失败: %v\n", err)
	// 		continue
	// 	}

	// 	fmt.Printf("AI> %s\n\n", response.Content)
	// 	// 保存模型回复
	// 	messages = append(messages, response)
	// }

	// // // 3. 调用 LLM
	// // response, err := chatModel.Generate(ctx, messages)
	// // if err != nil {
	// // 	log.Fatalf("调用模型失败: %v", err)
	// // }

	// // // 4. 输出模型回复
	// // fmt.Println(response.Content)
}
