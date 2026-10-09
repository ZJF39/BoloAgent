package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	maxSearches     = 2
	maxReads        = 4
	maxContentRunes = 6000
)

type searchInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"针对研究主题的搜索关键词，优先官方文档或论文"`
}
type readInput struct {
	SourceID string `json:"source_id" jsonschema:"required" jsonschema_description:"search_web 返回的来源编号，例如 S1"`
}
type searchOutput struct {
	Status  string   `json:"status"`
	Sources []Source `json:"sources"`
}
type readOutput struct {
	Status string  `json:"status"`
	Source *Source `json:"source,omitempty"`
}
type researchTools struct {
	client          *tavilyClient
	evidence        *Evidence
	searches, reads int
}

func newResearchTools(client *tavilyClient) *researchTools {
	return &researchTools{client: client, evidence: newEvidence()}
}

func (s *researchTools) tools() ([]tool.BaseTool, error) {
	// InferTool 从输入 Go struct 和标签推导 JSON Schema，并自动完成参数/结果 JSON 转换。
	search, err := utils.InferTool("search_web", "搜索公开网页，返回候选来源与摘要。摘要仅作线索；引用前必须调用 read_source。", s.search)
	if err != nil {
		return nil, err
	}
	read, err := utils.InferTool("read_source", "读取某个候选来源的正文，成功后才可引用。输入来源编号，不要编造 URL。", s.read)
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{search, read}, nil
}

func (s *researchTools) search(ctx context.Context, input *searchInput) (*searchOutput, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	if s.searches >= maxSearches {
		return nil, fmt.Errorf("搜索预算已用完，请从现有来源中筛选并结束研究")
	}
	s.searches++ // 失败的请求也消耗预算，避免无限重试。
	results, err := s.client.search(ctx, query)
	if err != nil {
		return nil, err
	}
	output := &searchOutput{Status: "empty", Sources: []Source{}}
	for _, item := range results {
		if source := s.evidence.add(item.Title, item.URL, truncate(item.Content, 800)); source != nil {
			output.Sources = append(output.Sources, *source)
		}
	}
	if len(output.Sources) > 0 {
		output.Status = "success"
	}
	return output, nil
}

func (s *researchTools) read(ctx context.Context, input *readInput) (*readOutput, error) {
	source, ok := s.evidence.byID[strings.TrimSpace(input.SourceID)]
	if !ok {
		return nil, fmt.Errorf("来源不存在，请使用搜索结果中的 source_id")
	}
	if source.Content != "" {
		return &readOutput{Status: "success", Source: source}, nil
	}
	if s.reads >= maxReads {
		return nil, fmt.Errorf("网页读取预算已用完，请使用已读取的证据结束研究")
	}
	s.reads++
	content, err := s.client.extract(ctx, source.URL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return &readOutput{Status: "empty"}, nil
	}
	// 存储的证据和模型看到的片段完全相同；截断后不能声称读取了整篇文章。
	source.Content = truncate(strings.TrimSpace(content), maxContentRunes)
	return &readOutput{Status: "success", Source: source}, nil
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "\n[内容已截断]"
}
