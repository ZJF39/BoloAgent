package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

type CalculatorInput struct {
	A float64 `json:"a" jsonschema:"required" jsonschema_description:"第一个数字"`

	B float64 `json:"b" jsonschema:"required" jsonschema_description:"第二个数字"`

	Operation string `json:"operation" jsonschema:"required,enum=add,enum=subtract,enum=multiply,enum=divide" jsonschema_description:"运算类型"`
}

type CalculatorOutput struct {
	Result float64 `json:"result"`
}

func calculator(ctx context.Context, input *CalculatorInput) (*CalculatorOutput, error) {

	var result float64

	switch input.Operation {

	case "add":
		result = input.A + input.B

	case "subtract":
		result = input.A - input.B

	case "multiply":
		result = input.A * input.B

	case "divide":
		if input.B == 0 {
			return nil, fmt.Errorf("除数不能为 0")
		}

		result = input.A / input.B

	default:
		return nil, fmt.Errorf(
			"不支持的运算类型: %s",
			input.Operation,
		)
	}

	return &CalculatorOutput{
		Result: result,
	}, nil
}

func main() {
	ctx := context.Background()

	// =========================
	// 1. 创建 Tool
	// =========================

	calculatorTool, err := utils.InferTool(
		"calculator",
		"执行两个数字之间的加、减、乘、除运算。遇到数学计算时使用。",
		calculator,
	)
	if err != nil {
		log.Fatal(err)
	}

	calculatorInfo, err := calculatorTool.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// =========================
	// 2. 创建模型
	// =========================

	chatModel, err := openai.NewChatModel(
		ctx,
		&openai.ChatModelConfig{
			APIKey:  os.Getenv("OPENAI_API_KEY"),
			Model:   os.Getenv("OPENAI_MODEL"),
			BaseURL: os.Getenv("OPENAI_BASE_URL"),
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	// =========================
	// 3. 把 Tool 注册给模型
	// =========================

	modelWithTools, err := chatModel.WithTools(
		[]*schema.ToolInfo{
			calculatorInfo,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	// =========================
	// 4. 初始消息
	// =========================

	messages := []*schema.Message{
		{
			Role: schema.System,
			Content: `
你是一名助手。

如果用户要求进行数学计算，
请使用 calculator 工具。

拿到工具结果以后，
根据结果回答用户。
`,
		},
		{
			Role:    schema.User,
			Content: "2*3",
		},
	}

	// =========================
	// 5. 第一次调用 LLM
	// =========================

	response, err := modelWithTools.Generate(
		ctx,
		messages,
	)
	if err != nil {
		log.Fatal(err)
	}

	// =========================
	// 6. 如果没有 Tool Call
	// =========================

	if len(response.ToolCalls) == 0 {
		fmt.Println("模型直接回答：")
		fmt.Println(response.Content)
		return
	}

	// =========================
	// 7. 保存 Assistant Message
	// =========================

	messages = append(messages, response)

	// =========================
	// 8. 执行每一个 Tool Call
	// =========================

	for _, toolCall := range response.ToolCalls {

		fmt.Println("模型请求调用工具:")
		fmt.Println("ID:", toolCall.ID)
		fmt.Println("Name:", toolCall.Function.Name)
		fmt.Println("Arguments:", toolCall.Function.Arguments)

		switch toolCall.Function.Name {

		case "calculator":

			// 真正执行 calculator Tool
			toolResult, err := calculatorTool.InvokableRun(
				ctx,
				toolCall.Function.Arguments,
			)

			if err != nil {
				log.Fatalf(
					"calculator 执行失败: %v",
					err,
				)
			}

			fmt.Println("Tool 执行结果:")
			fmt.Println(toolResult)

			// =========================
			// 9. 构造 Tool Message
			// =========================

			toolMessage := schema.ToolMessage(
				toolResult,
				toolCall.ID,
			)

			messages = append(
				messages,
				toolMessage,
			)

		default:
			log.Fatalf(
				"未知工具: %s",
				toolCall.Function.Name,
			)
		}
	}

	// =========================
	// 10. 把 Tool Result
	//     再次交给模型
	// =========================

	finalResponse, err := modelWithTools.Generate(
		ctx,
		messages,
	)
	if err != nil {
		log.Fatal(err)
	}

	// =========================
	// 11. 最终答案
	// =========================

	fmt.Println("\n模型最终回答:")
	fmt.Println(finalResponse.Content)
}
