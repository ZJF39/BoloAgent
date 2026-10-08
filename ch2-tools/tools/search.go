package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type SearchInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"互联网搜索关键词"`
}

type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type SearchOutput struct {
	Results []SearchResult `json:"results"`
}

func NewSearchTool() (tool.InvokableTool, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	return utils.InferTool(
		"search_web",
		"搜索互联网上的公开信息，返回网页标题、摘要和来源 URL。适用于最新信息或外部资料。",
		func(ctx context.Context, input *SearchInput) (*SearchOutput, error) {

			key := os.Getenv("BRAVE_SEARCH_API_KEY")
			if key == "" {
				return nil, fmt.Errorf("BRAVE_SEARCH_API_KEY 未设置")
			}

			query := strings.TrimSpace(input.Query)
			if query == "" {
				return nil, fmt.Errorf("搜索词不能为空")
			}

			endpoint, _ := url.Parse(
				"https://api.search.brave.com/res/v1/web/search",
			)

			q := endpoint.Query()
			q.Set("q", query)
			q.Set("count", "5")
			endpoint.RawQuery = q.Encode()

			req, err := http.NewRequestWithContext(
				ctx, http.MethodGet, endpoint.String(), nil,
			)
			if err != nil {
				return nil, err
			}

			req.Header.Set("Accept", "application/json")
			req.Header.Set("X-Subscription-Token", key)

			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
				return nil, fmt.Errorf(
					"搜索服务返回 %d: %s",
					resp.StatusCode, string(data),
				)
			}

			var raw struct {
				Web struct {
					Results []struct {
						Title       string `json:"title"`
						URL         string `json:"url"`
						Description string `json:"description"`
					} `json:"results"`
				} `json:"web"`
			}

			if err := json.NewDecoder(
				io.LimitReader(resp.Body, 1<<20),
			).Decode(&raw); err != nil {
				return nil, err
			}

			output := &SearchOutput{
				Results: make([]SearchResult, 0),
			}

			for _, item := range raw.Web.Results {
				output.Results = append(output.Results, SearchResult{
					Title:       item.Title,
					URL:         item.URL,
					Description: item.Description,
				})
			}

			return output, nil
		},
	)
}
