package tools

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type BrowserInput struct {
	URL string `json:"url" jsonschema:"required" jsonschema_description:"要读取的 HTTPS 网页 URL"`
}

type BrowserOutput struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func NewBrowserTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"browse_page",
		"使用浏览器打开并读取允许访问的公开网页。适用于查看搜索结果的实际网页内容。",
		func(ctx context.Context, input *BrowserInput) (*BrowserOutput, error) {

			u, err := url.Parse(input.URL)
			if err != nil {
				return nil, err
			}

			if u.Scheme != "https" || u.User != nil {
				return nil, fmt.Errorf("只允许 HTTPS URL")
			}

			// 教学示例：只允许访问明确列出的站点。
			allowed := map[string]bool{
				"go.dev":                   true,
				"www.cloudwego.io":         true,
				"datawhalechina.github.io": true,
				"example.com":              true,
			}

			if !allowed[u.Hostname()] {
				return nil, fmt.Errorf("不允许访问该站点: %s", u.Hostname())
			}

			browserCtx, cancel := context.WithTimeout(
				ctx, 12*time.Second,
			)
			defer cancel()

			browserCtx, browserCancel := chromedp.NewContext(browserCtx)
			defer browserCancel()

			// 1. 导航到目标页面，并等待 body 加载
			err = chromedp.Do(
				browserCtx,
				chromedp.Navigate(input.URL),
				chromedp.WaitReady(chromedp.CSS("body")),
			)
			if err != nil {
				return nil, fmt.Errorf("打开网页失败: %w", err)
			}

			// 2. 获取网页标题
			title, err := chromedp.Run(
				browserCtx,
				chromedp.Title(),
			)
			if err != nil {
				return nil, fmt.Errorf("获取网页标题失败: %w", err)
			}

			// 3. 获取网页正文
			body, err := chromedp.Run(
				browserCtx,
				chromedp.Text(chromedp.CSS("body")),
			)
			if err != nil {
				return nil, fmt.Errorf("获取网页内容失败: %w", err)
			}

			// 4. 限制内容长度
			chars := []rune(body)
			if len(chars) > 3000 {
				body = string(chars[:3000]) + "\n[内容已截断]"
			}

			return &BrowserOutput{
				URL:     input.URL,
				Title:   title,
				Content: body,
			}, nil
		},
	)
}
