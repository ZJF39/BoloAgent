package rag

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

type scoredDocument struct {
	Document *schema.Document
	Score    float64
}

// Retrieve 根据 Query 检索最相关的 TopK 文档。
func Retrieve(
	ctx context.Context,
	embedder embedding.Embedder,
	store *MemoryStore,
	query string,
	topK int,
) ([]*schema.Document, error) {

	if query == "" {
		return nil, fmt.Errorf("query 不能为空")
	}

	if topK <= 0 {
		return nil, fmt.Errorf(
			"topK 必须大于 0",
		)
	}

	// 1. Query → Vector
	queryVectors, err :=
		embedder.EmbedStrings(
			ctx,
			[]string{query},
		)

	if err != nil {
		return nil, fmt.Errorf(
			"Query Embedding 失败: %w",
			err,
		)
	}

	if len(queryVectors) != 1 {
		return nil, fmt.Errorf(
			"Query Embedding 返回数量异常",
		)
	}

	queryVector := queryVectors[0]

	// 2. 与所有 Document Vector 计算相似度
	results := make(
		[]scoredDocument,
		0,
		store.Size(),
	)

	for _, item := range store.Items() {

		score, err := cosineSimilarity(
			queryVector,
			item.Vector,
		)

		if err != nil {
			return nil, err
		}

		// 不直接修改 Store 内部的 Document，
		// 因为 score 是每次查询动态产生的。
		doc := cloneDocument(item.Document)

		doc.WithScore(score)

		results = append(
			results,
			scoredDocument{
				Document: doc,
				Score:    score,
			},
		)
	}

	// 3. Score 从高到低排序
	sort.Slice(
		results,
		func(i, j int) bool {
			return results[i].Score >
				results[j].Score
		},
	)

	// 4. TopK
	if topK > len(results) {
		topK = len(results)
	}

	docs := make(
		[]*schema.Document,
		0,
		topK,
	)

	for i := 0; i < topK; i++ {
		docs = append(
			docs,
			results[i].Document,
		)
	}

	return docs, nil
}

// cosineSimilarity 计算余弦相似度。
func cosineSimilarity(
	a []float64,
	b []float64,
) (float64, error) {

	if len(a) != len(b) {
		return 0, fmt.Errorf(
			"向量维度不一致: %d != %d",
			len(a),
			len(b),
		)
	}

	var dot float64
	var normA float64
	var normB float64

	for i := range a {
		dot += a[i] * b[i]

		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0, nil
	}

	return dot /
		(math.Sqrt(normA) *
			math.Sqrt(normB)), nil
}

// cloneDocument 避免修改 MemoryStore 里的原始 Document。
func cloneDocument(
	doc *schema.Document,
) *schema.Document {

	meta := make(map[string]any)

	for key, value := range doc.MetaData {
		meta[key] = value
	}

	return &schema.Document{
		ID:       doc.ID,
		Content:  doc.Content,
		MetaData: meta,
	}
}
