package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type tavilyClient struct {
	apiKey, baseURL string
	httpClient      *http.Client
}
type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

func newTavily(apiKey string) *tavilyClient {
	return &tavilyClient{apiKey: apiKey, baseURL: "https://api.tavily.com", httpClient: &http.Client{}}
}

func (c *tavilyClient) search(ctx context.Context, query string) ([]searchResult, error) {
	var response struct {
		Results []searchResult `json:"results"`
	}
	err := c.post(ctx, "/search", map[string]any{
		"query": query, "max_results": 5, "search_depth": "basic", "include_answer": false,
	}, &response)
	return response.Results, err
}

func (c *tavilyClient) extract(ctx context.Context, sourceURL string) (string, error) {
	var response struct {
		Results []struct {
			URL        string `json:"url"`
			RawContent string `json:"raw_content"`
		} `json:"results"`
		FailedResults []struct {
			Error string `json:"error"`
		} `json:"failed_results"`
	}
	if err := c.post(ctx, "/extract", map[string]any{
		"urls": []string{sourceURL}, "extract_depth": "basic", "format": "text",
	}, &response); err != nil {
		return "", err
	}
	// Extract 即使 HTTP 200 也可能逐 URL 失败，不能只检查 HTTP 状态。
	for _, result := range response.Results {
		if result.URL == sourceURL {
			return result.RawContent, nil
		}
	}
	if len(response.FailedResults) > 0 {
		return "", fmt.Errorf("网页正文提取失败，请选择其他来源")
	}
	return "", nil
}

func (c *tavilyClient) post(ctx context.Context, path string, payload any, output any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Tavily 请求失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Tavily %s 返回 HTTP %d（401 检查 Key，429 检查限流/配额）", path, response.StatusCode)
	}
	// 给响应大小设上限，避免意外的大网页占满内存。
	const maxBytes = 2 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxBytes {
		return fmt.Errorf("Tavily 响应超过 2 MiB")
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("Tavily 返回了无法解析的 JSON")
	}
	return nil
}
