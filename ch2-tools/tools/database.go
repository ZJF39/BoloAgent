package tools

import (
	"context"
	"database/sql"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	_ "modernc.org/sqlite"
)

type DatabaseInput struct {
	Keyword string `json:"keyword" jsonschema:"required" jsonschema_description:"要在笔记标题或内容中搜索的关键词"`
}

type Note struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type DatabaseOutput struct {
	Notes []Note `json:"notes"`
}

// 初始化演示数据库。
func OpenDemoDB(ctx context.Context) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "./notes.db")
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS notes (
            id INTEGER PRIMARY KEY,
            title TEXT NOT NULL,
            body TEXT NOT NULL
        )
    `)
	if err != nil {
		db.Close()
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
        INSERT OR IGNORE INTO notes(id, title, body)
        VALUES
        (1, 'Agent Loop', 'Agent Loop 包含观察、决策、行动和反馈'),
        (2, 'RAG', 'RAG 包含 Chunk、Embedding、Retrieve 和 Generate'),
        (3, 'Eino', 'Eino 是 Go 语言的 AI 应用开发框架')
    `)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func NewDatabaseTool(db *sql.DB) (tool.InvokableTool, error) {
	return utils.InferTool(
		"query_database",
		"在本地 notes 数据库中按关键词搜索笔记。只能读取，不能修改。",
		func(ctx context.Context, input *DatabaseInput) (*DatabaseOutput, error) {

			keyword := strings.TrimSpace(input.Keyword)
			if keyword == "" {
				return &DatabaseOutput{Notes: []Note{}}, nil
			}

			pattern := "%" + keyword + "%"

			rows, err := db.QueryContext(ctx, `
                SELECT id, title, body
                FROM notes
                WHERE title LIKE ? OR body LIKE ?
                LIMIT 5
            `, pattern, pattern)
			if err != nil {
				return nil, err
			}
			defer rows.Close()

			result := &DatabaseOutput{
				Notes: make([]Note, 0),
			}

			for rows.Next() {
				var note Note

				if err := rows.Scan(
					&note.ID,
					&note.Title,
					&note.Body,
				); err != nil {
					return nil, err
				}

				result.Notes = append(result.Notes, note)
			}

			if err := rows.Err(); err != nil {
				return nil, err
			}

			return result, nil
		},
	)
}
