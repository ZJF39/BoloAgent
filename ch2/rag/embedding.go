package rag

import (
	"context"
	"fmt"
	"os"
	"time"

	embeddingOpenAI "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// NewEmbedder 创建 Embedding 模型。
//
// 注意：
// DeepSeek Chat API 不提供 Embedding，
// 所以这里单独使用一个 Embedding Provider。
func NewEmbedder(
	ctx context.Context,
) (embedding.Embedder, error) {

	apiKey := os.Getenv("EMBEDDING_API_KEY")
	modelName := os.Getenv("EMBEDDING_MODEL")

	if apiKey == "" {
		return nil, fmt.Errorf(
			"环境变量 EMBEDDING_API_KEY 未设置",
		)
	}

	if modelName == "" {
		return nil, fmt.Errorf(
			"环境变量 EMBEDDING_MODEL 未设置",
		)
	}

	embedder, err :=
		embeddingOpenAI.NewEmbedder(
			ctx,
			&embeddingOpenAI.EmbeddingConfig{
				APIKey:  apiKey,
				Model:   modelName,
				BaseURL: os.Getenv("EMBEDDING_BASE_URL"),
				Timeout: 30 * time.Second,
			},
		)

	if err != nil {
		return nil, fmt.Errorf(
			"创建 Embedder 失败: %w",
			err,
		)
	}

	return embedder, nil
}

// EmbedDocuments 将所有 chunk 转换成向量。
func EmbedDocuments(
	ctx context.Context,
	embedder embedding.Embedder,
	docs []*schema.Document,
) ([][]float64, error) {

	texts := make(
		[]string,
		0,
		len(docs),
	)

	for _, doc := range docs {
		texts = append(
			texts,
			doc.Content,
		)
	}

	vectors, err := embedder.EmbedStrings(
		ctx,
		texts,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"文档 Embedding 失败: %w",
			err,
		)
	}

	if len(vectors) != len(docs) {
		return nil, fmt.Errorf(
			"Embedding 数量与 Document 数量不一致: docs=%d vectors=%d",
			len(docs),
			len(vectors),
		)
	}

	return vectors, nil
}
