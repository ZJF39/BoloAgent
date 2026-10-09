package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

func research(ctx context.Context, topic string, researcher model.ToolCallingChatModel, writer model.BaseChatModel, client *tavilyClient) (string, error) {
	// 每次研究创建独立状态。这是本次任务的证据表，不是长期记忆。
	state := newResearchTools(client)
	tools, err := state.tools()
	if err != nil {
		return "", err
	}
	// ReAct 自动完成：模型决策 → Tool Call → ToolsNode → ToolMessage → 模型。
	// 无需手写 Agent Loop，也无需自己绑定 ToolCall ID。
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: researcher,
		MaxStep:          20, // Graph 的执行步数；一次模型 + 工具循环约占两步。
		ToolsConfig: compose.ToolsNodeConfig{
			Tools:               tools,
			ExecuteSequentially: true, // 状态使用普通 map，因此工具按顺序执行。
			ToolCallMiddlewares: []compose.ToolMiddleware{{Invokable: reliabilityMiddleware()}},
			UnknownToolsHandler: func(ctx context.Context, _, _ string) (string, error) {
				return `{"status":"error","message":"工具不存在，请使用 search_web 或 read_source"}`, ctx.Err()
			},
		},
	})
	if err != nil {
		return "", err
	}
	_, err = agent.Generate(ctx, []*schema.Message{
		schema.SystemMessage(researchPrompt), schema.UserMessage("研究主题：" + topic),
	})
	if err != nil {
		return "", fmt.Errorf("Eino 研究过程: %w", err)
	}
	sources := state.evidence.readSources()
	if len(sources) == 0 {
		return "# " + escapeMarkdown(topic) + "\n\n没有成功读取可引用的网页正文，无法给出有依据的总结。请查看工具日志，调整主题或检查 API 配置。\n", nil
	}
	// 写作只接收成功读取的正文，不把搜索摘要或 Agent 的中间猜测当成证据。
	data, err := json.Marshal(struct {
		Topic   string   `json:"topic"`
		Sources []Source `json:"sources"`
	}{topic, sources})
	if err != nil {
		return "", err
	}
	messages := []*schema.Message{schema.SystemMessage(writePrompt), schema.UserMessage(string(data))}
	// 最多修正一次格式或引用问题；校验仍失败就返回错误，不输出未经检查的报告。
	for attempt := 0; attempt < 2; attempt++ {
		response, err := writer.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("生成研究报告: %w", err)
		}
		report, err := parseReport(response.Content, state.evidence)
		if err == nil {
			return report.markdown(topic, state.evidence), nil
		}
		if attempt == 1 {
			return "", fmt.Errorf("报告校验失败: %w", err)
		}
		log.Printf("[报告校验] %v；尝试修正一次", err)
		messages = append(messages, response, schema.UserMessage("请依据原始证据修正 JSON。校验错误："+err.Error()))
	}
	return "", fmt.Errorf("报告生成未完成")
}

// Middleware 是 Eino 工具执行的拦截层，类似 HTTP Middleware。
// 只包装普通工具错误；任务取消必须继续返回 error，让 Eino 停止运行。
func reliabilityMiddleware() compose.InvokableToolMiddleware {
	counts := map[string]int{}
	cache := map[string]*compose.ToolOutput{}
	return func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
		return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			var args map[string]any
			if json.Unmarshal([]byte(input.Arguments), &args) != nil || args == nil {
				return observation("error", "参数必须是 JSON Object"), nil
			}
			normalized, _ := json.Marshal(args) // map 键排序，也规范化嵌套对象的键顺序。
			key := input.Name + ":" + string(normalized)
			counts[key]++
			if counts[key] > 2 {
				return observation("blocked", "重复调用过多，请换方法或结束研究"), nil
			}
			if cached, ok := cache[key]; ok {
				log.Printf("[工具] %s 缓存命中", input.Name)
				return cached, nil
			}
			started := time.Now()
			toolCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
			defer cancel()
			output, err := next(toolCtx, input)
			log.Printf("[工具] %s 耗时=%s 成功=%t", input.Name, time.Since(started).Round(time.Millisecond), err == nil)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if err != nil {
				return observation("error", err.Error()), nil
			}
			// 本例两种工具均只读，因此允许在当前任务中复用成功结果。
			cache[key] = output
			return output, nil
		}
	}
}
func observation(status, message string) *compose.ToolOutput {
	data, _ := json.Marshal(map[string]string{"status": status, "message": message})
	return &compose.ToolOutput{Result: string(data)}
}
