package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

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

	// 2. 构造对话消息
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "你是一名专业的软件工程助手。",
		},
		// {
		// 	Role:    schema.User,
		// 	Content: "请简单解释什么是 Agent。",
		// },
	}
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("You> ")
		if !scanner.Scan() {
			break
		}
		/**
		 * @description: 获取用户输入
		 * TrimSpace 函数用于去除输入字符串首尾的空白字符。
		 */
		input := strings.TrimSpace(scanner.Text())

		if input == "exit" {
			break
		}
		// 加入用户信息
		messages = append(messages, &schema.Message{
			Role:    schema.User,
			Content: input,
		})
		// 调用模型
		response, err := chatModel.Generate(ctx, messages)
		if err != nil {
			log.Printf("模型调用失败: %v\n", err)
			continue
		}

		fmt.Printf("AI> %s\n\n", response.Content)
		// 保存模型回复
		messages = append(messages, response)
	}

	// // 3. 调用 LLM
	// response, err := chatModel.Generate(ctx, messages)
	// if err != nil {
	// 	log.Fatalf("调用模型失败: %v", err)
	// }

	// // 4. 输出模型回复
	// fmt.Println(response.Content)
}
