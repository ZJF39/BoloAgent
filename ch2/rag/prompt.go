package rag

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// BuildRAGMessages 根据检索结果构造最终 Prompt。
func BuildRAGMessages(
	question string,
	docs []*schema.Document,
) []*schema.Message {

	contextText := buildCitationContext(docs)

	systemPrompt := `
你是一个基于知识库回答问题的助手。

必须严格遵守以下规则：

1. 只能根据提供的检索资料回答问题。
2. 不要使用资料之外的信息进行猜测。
3. 如果资料不足以回答问题，请明确说：
   "根据当前知识库资料无法确定。"
4. 每个事实都应尽量使用 [1]、[2] 等编号标注来源。
5. 引用编号只能使用提供给你的资料编号。
6. 不要编造不存在的引用。
`

	userPrompt := fmt.Sprintf(`
下面是从知识库检索到的资料：

%s

用户问题：

%s

请根据以上资料回答，并保留引用编号。
`,
		contextText,
		question,
	)

	return []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.User,
			Content: userPrompt,
		},
	}
}

// buildCitationContext 将 Document 转换成带编号的上下文。
func buildCitationContext(
	docs []*schema.Document,
) string {

	var builder strings.Builder

	for i, doc := range docs {

		source := "unknown"
		chunk := "unknown"

		if value, ok :=
			doc.MetaData["source"]; ok {
			source = fmt.Sprint(value)
		}

		if value, ok :=
			doc.MetaData["chunk"]; ok {
			chunk = fmt.Sprint(value)
		}

		fmt.Fprintf(
			&builder,
			"[%d]\n来源: %s\nChunk: %s\n相关度: %.4f\n内容:\n%s\n\n",
			i+1,
			source,
			chunk,
			doc.Score(),
			doc.Content,
		)
	}

	return builder.String()
}
