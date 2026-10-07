package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/schema"
)

// LoadMarkdownDocuments 读取目录中的所有 Markdown 文件。
func LoadMarkdownDocuments(dir string) ([]*schema.Document, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取文档目录失败: %w", err)
	}

	var docs []*schema.Document

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		path := filepath.Join(dir, entry.Name())

		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf(
				"读取文件 %s 失败: %w",
				path,
				err,
			)
		}

		doc := &schema.Document{
			ID:      entry.Name(),
			Content: string(content),
			MetaData: map[string]any{
				"source": entry.Name(),
			},
		}

		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf(
			"目录 %s 中没有找到 Markdown 文件",
			dir,
		)
	}

	return docs, nil
}

// SplitDocuments 使用 Eino Recursive Splitter 将文档切块。
func SplitDocuments(
	ctx context.Context,
	docs []*schema.Document,
	chunkSize int,
	overlapSize int,
) ([]*schema.Document, error) {

	splitter, err := recursive.NewSplitter(
		ctx,
		&recursive.Config{
			ChunkSize:   chunkSize,
			OverlapSize: overlapSize,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"创建 splitter 失败: %w",
			err,
		)
	}

	var allChunks []*schema.Document

	// 一份文档一份文档地切，
	// 方便给 chunk 编号。
	for _, doc := range docs {

		chunks, err := splitter.Transform(
			ctx,
			[]*schema.Document{doc},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"切分文档 %s 失败: %w",
				doc.ID,
				err,
			)
		}

		for i, chunk := range chunks {

			if chunk.MetaData == nil {
				chunk.MetaData = make(map[string]any)
			}

			source := doc.ID

			if value, ok :=
				doc.MetaData["source"].(string); ok {
				source = value
			}

			chunk.ID = fmt.Sprintf(
				"%s#chunk-%d",
				source,
				i+1,
			)

			chunk.MetaData["source"] = source
			chunk.MetaData["chunk"] = i + 1

			allChunks = append(
				allChunks,
				chunk,
			)
		}
	}

	return allChunks, nil
}
