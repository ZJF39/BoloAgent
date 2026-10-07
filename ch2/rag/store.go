package rag

import (
	"fmt"

	"github.com/cloudwego/eino/schema"
)

// VectorItem 表示一个 Document 和它对应的向量。
type VectorItem struct {
	Document *schema.Document
	Vector   []float64
}

// MemoryStore 是最小内存向量库。
type MemoryStore struct {
	items []VectorItem
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make([]VectorItem, 0),
	}
}

// Add 将 Document 和对应的 Vector 保存到内存。
func (s *MemoryStore) Add(
	docs []*schema.Document,
	vectors [][]float64,
) error {

	if len(docs) != len(vectors) {
		return fmt.Errorf(
			"Document 和 Vector 数量不一致: docs=%d vectors=%d",
			len(docs),
			len(vectors),
		)
	}

	for i, doc := range docs {

		// 向量复制一份，避免外部修改。
		vectorCopy := append(
			[]float64(nil),
			vectors[i]...,
		)

		s.items = append(
			s.items,
			VectorItem{
				Document: doc,
				Vector:   vectorCopy,
			},
		)
	}

	return nil
}

func (s *MemoryStore) Items() []VectorItem {
	return s.items
}

func (s *MemoryStore) Size() int {
	return len(s.items)
}
