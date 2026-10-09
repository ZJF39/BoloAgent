package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type Source struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	Content string `json:"content,omitempty"`
}
type Evidence struct {
	byID    map[string]*Source
	byURL   map[string]*Source
	ordered []*Source
}

func newEvidence() *Evidence {
	return &Evidence{byID: map[string]*Source{}, byURL: map[string]*Source{}}
}

func (e *Evidence) add(title, rawURL, snippet string) *Source {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return nil
	}
	u.Fragment = "" // 同一网页的不同锚点不重复编号。
	canonical := u.String()
	if existing := e.byURL[canonical]; existing != nil {
		return existing
	}
	source := &Source{ID: fmt.Sprintf("S%d", len(e.ordered)+1), Title: title, URL: canonical, Snippet: snippet}
	e.byID[source.ID], e.byURL[canonical] = source, source
	e.ordered = append(e.ordered, source)
	return source
}

func (e *Evidence) readSources() []Source {
	var sources []Source
	for _, source := range e.ordered {
		if source.Content != "" {
			copy := *source
			copy.Snippet = "" // 写作阶段移除搜索摘要，只留下正文与来源信息。
			sources = append(sources, copy)
		}
	}
	return sources
}

// 模型只输出结论与来源 ID。真实 URL 由程序从证据表查找，不让模型生成。
type Finding struct {
	Text      string   `json:"text"`
	SourceIDs []string `json:"source_ids"`
}
type Report struct {
	Findings    []Finding `json:"findings"`
	Limitations []string  `json:"limitations"`
}

func parseReport(raw string, evidence *Evidence) (*Report, error) {
	var report Report
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return nil, fmt.Errorf("必须返回符合约定的 JSON 对象")
	}
	if len(report.Findings) == 0 && len(report.Limitations) == 0 {
		return nil, fmt.Errorf("没有结论时必须说明证据不足的原因")
	}
	for _, limitation := range report.Limitations {
		if strings.TrimSpace(limitation) == "" {
			return nil, fmt.Errorf("局限说明不能为空")
		}
	}
	for i, finding := range report.Findings {
		if strings.TrimSpace(finding.Text) == "" || len(finding.SourceIDs) == 0 {
			return nil, fmt.Errorf("结论 %d 缺少正文或引用", i+1)
		}
		if strings.Contains(finding.Text, "://") {
			return nil, fmt.Errorf("结论正文不要生成 URL，请通过 source_ids 引用")
		}
		for _, id := range finding.SourceIDs {
			source := evidence.byID[id]
			if source == nil || source.Content == "" {
				return nil, fmt.Errorf("引用 %s 不存在或没有成功读取正文", id)
			}
		}
	}
	return &report, nil
}

// 此校验保证编号存在、已读取正文、每条结论都有引用；不保证语义上支持结论。
// Grounding 判断仍由读者对照原文；后续可在这里加入逐结论的证据一致性检查。
func (r *Report) markdown(topic string, evidence *Evidence) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n## 研究结论\n\n", escapeMarkdown(topic))
	used := map[string]bool{}
	if len(r.Findings) == 0 {
		b.WriteString("当前证据不足，无法形成有依据的结论。\n\n")
	}
	for _, finding := range r.Findings {
		fmt.Fprintf(&b, "- %s", escapeMarkdown(finding.Text))
		seen := map[string]bool{}
		for _, id := range finding.SourceIDs {
			if seen[id] {
				continue
			}
			seen[id], used[id] = true, true
			fmt.Fprintf(&b, " [%s](<%s>)", id, evidence.byID[id].URL)
		}
		b.WriteString("\n")
	}
	if len(r.Limitations) > 0 {
		b.WriteString("\n## 不确定性与局限\n\n")
		for _, limitation := range r.Limitations {
			fmt.Fprintf(&b, "- %s\n", escapeMarkdown(limitation))
		}
	}
	b.WriteString("\n## 引用来源\n\n")
	for _, source := range evidence.ordered {
		if used[source.ID] {
			fmt.Fprintf(&b, "- [%s] [%s](<%s>)\n", source.ID, escapeMarkdown(source.Title), source.URL)
		}
	}
	b.WriteString("\n> 引用已做来源编号与正文读取检查，尚未做语义一致性校验。正文每页最多保留 6000 字符；重要结论请打开链接核对。\n")
	return b.String()
}

func escapeMarkdown(text string) string {
	return strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "`", "\\`", "<", "&lt;", ">", "&gt;", "\n", " ", "\r", " ").Replace(text)
}
