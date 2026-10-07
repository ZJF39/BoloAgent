package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
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

func makeToolError(message string) string {

	data := struct {
		Error string `json:"error"`
	}{
		Error: message,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return `{"error":"unknown tool error"}`
	}

	return string(b)
}
func runAgent(
	ctx context.Context,
	modelWithTools model.ToolCallingChatModel,
	tools map[string]tool.InvokableTool,
	messages []*schema.Message,
	maxSteps int,
) error {

	for step := 1; step <= maxSteps; step++ {

		fmt.Printf("\n===== Step %d/%d =====\n", step, maxSteps)

		// 1. 先检查整个 Agent 是否已经超时
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(
				"agent 已被取消或超时: %w",
				err,
			)
		}

		// 2. 让模型决定下一步
		response, err := modelWithTools.Generate(
			ctx,
			messages,
		)
		if err != nil {
			return fmt.Errorf(
				"第 %d 步模型调用失败: %w",
				step,
				err,
			)
		}

		// Assistant 消息必须保存
		messages = append(messages, response)

		// 3. 没有 ToolCall = Agent 给出最终答案
		if len(response.ToolCalls) == 0 {

			if response.Content == "" {
				return fmt.Errorf(
					"第 %d 步模型既没有返回 ToolCall，也没有返回文本",
					step,
				)
			}

			fmt.Println("\nAgent 最终回答:")
			fmt.Println(response.Content)

			return nil
		}

		// 4. 执行所有 ToolCall
		for _, toolCall := range response.ToolCalls {

			fmt.Printf(
				"调用工具: %s\n",
				toolCall.Function.Name,
			)

			fmt.Printf(
				"参数: %s\n",
				toolCall.Function.Arguments,
			)

			selectedTool, ok :=
				tools[toolCall.Function.Name]

			// 找不到工具
			if !ok {

				result := makeToolError(
					"未知工具: " +
						toolCall.Function.Name,
				)

				messages = append(
					messages,
					schema.ToolMessage(
						result,
						toolCall.ID,
					),
				)

				continue
			}

			// 5. 真正执行 Tool
			result, err := selectedTool.InvokableRun(
				ctx,
				toolCall.Function.Arguments,
			)

			// 6. Tool 的业务错误不要立刻杀死 Agent
			if err != nil {

				fmt.Println(
					"工具执行失败:",
					err,
				)

				result = makeToolError(
					err.Error(),
				)
			}

			// 7. Tool Result 作为 Observation
			messages = append(
				messages,
				schema.ToolMessage(
					result,
					toolCall.ID,
				),
			)
		}
	}

	return fmt.Errorf(
		"agent 达到最大步数 %d，仍未完成任务",
		maxSteps,
	)
}
func main() {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

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
	tools := map[string]tool.InvokableTool{
		"calculator": calculatorTool,
	}

	messages := []*schema.Message{
		{
			Role: schema.System,
			Content: `
你是一名可以使用工具的 Agent。

如果需要数学计算，请调用 calculator。

工具发生错误时，请根据错误信息决定：
- 是否修改参数重新调用工具；
- 是否可以直接向用户解释错误；
- 或者是否需要采用其他方式完成任务。
`,
		},
		{
			Role:    schema.User,
			Content: "帮我计算 23 * 47+7+3*5+7+8*8+8*8/3+9/10",
		},
	}

	err = runAgent(
		ctx,
		modelWithTools,
		tools,
		messages,
		5,
	)

	if err != nil {

		if errors.Is(
			err,
			context.DeadlineExceeded,
		) {
			log.Println("Agent 执行超时")
			return
		}

		log.Println("Agent 执行失败:", err)
	}
}
