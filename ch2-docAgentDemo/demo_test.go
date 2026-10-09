package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/compose"
)

// 使用真实 Eino 和 DeepSeek 适配器，只有远程 HTTP 响应被本地服务替代。
func TestResearchEndToEnd(t *testing.T) {
	for _, scenario := range []string{"normal", "repair", "invalid", "empty"} {
		t.Run(scenario, func(t *testing.T) {
			researchCalls, writerCalls, searches, reads := 0, 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/search":
					searches++
					if r.Header.Get("Authorization") != "Bearer test-key" {
						t.Error("搜索请求缺少认证")
					}
					if scenario == "empty" {
						fmt.Fprint(w, `{"results":[]}`)
						return
					}
					fmt.Fprint(w, `{"results":[{"title":"Eino 官方文档","url":"https://www.cloudwego.io/docs/eino/","content":"只作搜索线索"},{"title":"Eino 源码","url":"https://github.com/cloudwego/eino","content":"第二条搜索线索"}]}`)
				case "/extract":
					reads++
					var request struct {
						URLs []string `json:"urls"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.URLs) != 1 {
						t.Error("提取请求不正确")
						return
					}
					json.NewEncoder(w).Encode(map[string]any{"results": []map[string]string{{"url": request.URLs[0], "raw_content": "Eino ReAct 自动执行模型与工具的循环。"}}})
				case "/chat/completions":
					var request struct {
						ResponseFormat *struct {
							Type string `json:"type"`
						} `json:"response_format"`
						Tools    []json.RawMessage `json:"tools"`
						Messages []struct {
							Role       string `json:"role"`
							ToolCallID string `json:"tool_call_id"`
							Content    string `json:"content"`
						} `json:"messages"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						return
					}
					if request.ResponseFormat != nil && request.ResponseFormat.Type == "json_object" {
						writerCalls++
						if len(request.Tools) != 0 {
							t.Error("写作模型不应绑定工具")
						}
						if !strings.Contains(request.Messages[1].Content, "content") {
							t.Error("写作没有接收到正文证据")
						}
						if strings.Contains(request.Messages[1].Content, "搜索线索") {
							t.Error("写作上下文不应包含搜索摘要")
						}
						id := "S1"
						if scenario == "invalid" || (scenario == "repair" && writerCalls == 1) {
							id = "S999"
						}
						completion(w, fmt.Sprintf(`{"findings":[{"text":"Eino ReAct 自动执行工具循环。","source_ids":[%q]}],"limitations":["仅核对本次读取的资料。"]}`, id), nil)
						return
					}
					researchCalls++
					if len(request.Tools) != 2 {
						t.Errorf("期待两种注册工具，实际 %d", len(request.Tools))
					}
					if researchCalls == 1 {
						completion(w, "", []map[string]any{toolCall("call-search", "search_web", `{"query":"Eino ReAct 官方文档"}`)})
					} else if researchCalls == 2 {
						found := false
						for _, message := range request.Messages {
							if message.Role == "tool" && message.ToolCallID == "call-search" {
								found = true
							}
						}
						if !found {
							t.Error("Eino 没有追加与搜索调用对应的 ToolMessage")
						}
						if scenario == "empty" {
							completion(w, "资料收集结束", nil)
							return
						}
						completion(w, "", []map[string]any{toolCall("call-read-1", "read_source", `{"source_id":"S1"}`), toolCall("call-read-2", "read_source", `{"source_id":"S2"}`)})
					} else {
						found := map[string]bool{}
						for _, message := range request.Messages {
							if message.Role == "tool" {
								found[message.ToolCallID] = true
							}
						}
						if !found["call-read-1"] || !found["call-read-2"] {
							t.Error("缺少正文读取 ToolMessage")
						}
						completion(w, "资料收集结束", nil)
					}
				default:
					t.Errorf("不应请求 %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cfg := &deepseek.ChatModelConfig{APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-chat"}
			researcher, err := deepseek.NewChatModel(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			writerCfg := *cfg
			writerCfg.ResponseFormatType = deepseek.ResponseFormatTypeJSONObject
			writer, err := deepseek.NewChatModel(ctx, &writerCfg)
			if err != nil {
				t.Fatal(err)
			}
			client := newTavily("test-key")
			client.baseURL = server.URL
			report, err := research(ctx, "学习 Eino", researcher, writer, client)
			if scenario == "invalid" {
				if err == nil || report != "" || writerCalls != 2 {
					t.Fatalf("应在一次修正后拒绝无效引用: calls=%d err=%v", writerCalls, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "empty" {
				if writerCalls != 0 || reads != 0 || !strings.Contains(report, "无法给出有依据的总结") {
					t.Fatal("空结果不应进入写作")
				}
				return
			}
			if searches != 1 || reads != 2 {
				t.Fatalf("工具未自动执行: searches=%d reads=%d", searches, reads)
			}
			if !strings.Contains(report, "[S1](<https://www.cloudwego.io/docs/eino/>)") {
				t.Fatal("报告缺少真实引用链接")
			}
			if strings.Contains(report, "[S2]") {
				t.Fatal("未引用的来源不应出现在来源列表")
			}
			if scenario == "repair" && writerCalls != 2 {
				t.Fatal("错误引用应仅修正一次")
			}
		})
	}
}

func toolCall(id, name, args string) map[string]any {
	return map[string]any{"id": id, "type": "function", "function": map[string]string{"name": name, "arguments": args}}
}
func completion(w http.ResponseWriter, content string, calls []map[string]any) {
	json.NewEncoder(w).Encode(map[string]any{
		"id": "test", "object": "chat.completion", "model": "deepseek-chat",
		"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": content, "tool_calls": calls}, "finish_reason": "stop"}},
		"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
	})
}

func TestReportValidation(t *testing.T) {
	e := newEvidence()
	source := e.add("文档", "https://example.org/doc#section", "摘要")
	if duplicate := e.add("同一页面", "https://example.org/doc#other", "摘要"); duplicate.ID != source.ID {
		t.Fatal("网页锚点应去重")
	}
	if e.add("无效来源", "javascript:alert(1)", "") != nil {
		t.Fatal("不应接受非 HTTP URL")
	}
	valid := `{"findings":[{"text":"结论","source_ids":["S1"]}],"limitations":[]}`
	if _, err := parseReport(valid, e); err == nil {
		t.Fatal("搜索摘要不应成为可引用正文")
	}
	source.Content = "正文证据"
	if _, err := parseReport(valid, e); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"findings":[{"text":"结论","source_ids":["S999"]}]}`,
		`{"findings":[{"text":"无引用结论","source_ids":[]}]}`,
		`{"findings":[],"limitations":[]}`, `not json`,
		`{"findings":[],"limitations":[""]}`,
	} {
		if _, err := parseReport(raw, e); err == nil {
			t.Errorf("应拒绝 %s", raw)
		}
	}
}

func TestReliabilityMiddleware(t *testing.T) {
	executions := 0
	endpoint := reliabilityMiddleware()(func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		executions++
		return &compose.ToolOutput{Result: `{"status":"success"}`}, nil
	})
	for _, args := range []string{`{"a":1,"nested":{"x":1,"y":2}}`, `{"nested":{"y":2,"x":1},"a":1}`} {
		if _, err := endpoint(context.Background(), &compose.ToolInput{Name: "search_web", Arguments: args}); err != nil {
			t.Fatal(err)
		}
	}
	if executions != 1 {
		t.Fatal("相同调用应复用只读缓存")
	}
	blocked, err := endpoint(context.Background(), &compose.ToolInput{Name: "search_web", Arguments: `{"a":1,"nested":{"x":1,"y":2}}`})
	if err != nil || !strings.Contains(blocked.Result, "blocked") {
		t.Fatal("第三次重复调用应被阻止")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := endpoint(ctx, &compose.ToolInput{}); !errors.Is(err, context.Canceled) {
		t.Fatal("取消不能转为普通错误 Observation")
	}
	failed := reliabilityMiddleware()(func(context.Context, *compose.ToolInput) (*compose.ToolOutput, error) {
		return nil, errors.New("工具失败")
	})
	output, err := failed(context.Background(), &compose.ToolInput{Name: "read_source", Arguments: `{}`})
	if err != nil || !strings.Contains(output.Result, "error") {
		t.Fatal("普通错误应反馈给模型")
	}
}

func TestTavilyFailuresAndBudgets(t *testing.T) {
	for _, scenario := range []string{"rate-limit", "failed-extract", "empty-extract"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "rate-limit" {
					w.WriteHeader(429)
					return
				}
				if scenario == "failed-extract" {
					fmt.Fprint(w, `{"results":[],"failed_results":[{"error":"unavailable"}]}`)
					return
				}
				fmt.Fprint(w, `{"results":[]}`)
			}))
			defer server.Close()
			client := newTavily("test-key")
			client.baseURL = server.URL
			_, err := client.extract(context.Background(), "https://example.org/doc")
			if scenario != "empty-extract" && err == nil {
				t.Fatal("不能将 API 失败视为成功证据")
			}
			state := newResearchTools(client)
			s := state.evidence.add("文档", "https://example.org/doc", "摘要")
			_, _ = state.read(context.Background(), &readInput{SourceID: s.ID})
			if len(state.evidence.readSources()) != 0 {
				t.Fatal("空/失败正文不应进入证据表")
			}
			state.searches, state.reads = maxSearches, maxReads
			if _, err := state.search(context.Background(), &searchInput{Query: "主题"}); err == nil {
				t.Fatal("应限制搜索预算")
			}
			if _, err := state.read(context.Background(), &readInput{SourceID: s.ID}); err == nil {
				t.Fatal("应限制正文读取预算")
			}
		})
	}
}
