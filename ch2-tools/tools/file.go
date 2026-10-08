package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type ReadFileInput struct {
	Path string `json:"path" jsonschema:"required" jsonschema_description:"workspace 内的相对文件路径"`
}

type ReadFileOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

func NewFileTool(workspace string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"read_file",
		"只读工具。读取 workspace 目录内的 Markdown、文本或 Go 源文件。",
		func(ctx context.Context, input *ReadFileInput) (*ReadFileOutput, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			path := input.Path
			if !filepath.IsLocal(path) {
				return nil, fmt.Errorf("不允许访问非本地相对路径")
			}

			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".txt" && ext != ".go" {
				return nil, fmt.Errorf("不支持的文件类型: %s", ext)
			}

			root, err := os.OpenRoot(workspace)
			if err != nil {
				return nil, err
			}
			defer root.Close()

			file, err := root.Open(path)
			if err != nil {
				return nil, err
			}
			defer file.Close()

			info, err := file.Stat()
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("只允许读取普通文件")
			}

			const maxBytes = 12 * 1024
			data, err := io.ReadAll(io.LimitReader(file, maxBytes))
			if err != nil {
				return nil, err
			}

			return &ReadFileOutput{
				Path:      path,
				Content:   string(data),
				Truncated: info.Size() > maxBytes,
			}, nil
		},
	)
}
