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
	})
	if err != nil {
		log.Fatalf("创建 ChatModel 失败: %v", err)
	}

	// 2. 构造对话消息
	messages := []*schema.Message{
		{
			Role: schema.System,
			Content: `你是一个请求分类器。

请分析用户请求，并且只返回 JSON。

JSON 格式：

{
  "intent": "用户意图",
  "need_tool": true 或 false,
  "tool": "calculator、search 或空字符串",
  "query": "需要交给工具处理的内容"
}

不要输出 Markdown格式代码块。
不要输出解释。`,
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

	fmt.Println("模型原始输出：")
	fmt.Println(response.Content)

	var decision Decision

	// JSON反序列化
	err = json.Unmarshal(
		[]byte(response.Content),
		&decision,
	)

	if err != nil {
		log.Fatalf("JSON 解析失败: %v", err)
	}

	fmt.Printf("\n解析后的结构体：%+v\n", decision)

	fmt.Println("Intent:", decision.Intent)
	fmt.Println("NeedTool:", decision.NeedTool)
	fmt.Println("Tool:", decision.Tool)
	fmt.Println("Query:", decision.Query)

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
