package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

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
	fmt.Println(os.Getenv("OPENAI_API_KEY"))
	fmt.Println(os.Getenv("OPENAI_MODEL"))
	fmt.Println(os.Getenv("OPENAI_BASE_URL"))

	// 2. 构造对话消息
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "你是一名专业的软件工程助手。",
		},
		{
			Role:    schema.User,
			Content: "请简单解释什么是 Agent。",
		},
	}

	// 3. 调用 LLM
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("调用模型失败: %v", err)
	}

	// 4. 输出模型回复
	fmt.Println(response.Content)
}
