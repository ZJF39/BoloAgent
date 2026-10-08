package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type CodeInput struct {
	Code string `json:"code" jsonschema:"required" jsonschema_description:"需要在隔离容器中运行的 Python 代码"`
}

type CodeOutput struct {
	Output string `json:"output"`
}

// 限制进程输出占用的内存。
type boundedWriter struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	limit int
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n := len(p)

	remaining := w.limit - w.buf.Len()
	if remaining > 0 {
		if remaining > n {
			remaining = n
		}
		_, _ = w.buf.Write(p[:remaining])
	}

	return n, nil
}

func (w *boundedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func NewCodeTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"execute_code",
		"在受限制的 Docker 容器中执行短小的 Python 代码。适用于计算、数据处理和验证算法。",
		func(ctx context.Context, input *CodeInput) (*CodeOutput, error) {

			code := strings.TrimSpace(input.Code)

			if code == "" {
				return nil, fmt.Errorf("代码不能为空")
			}

			if len(code) > 2000 {
				return nil, fmt.Errorf("代码长度不能超过 2000 字节")
			}

			runCtx, cancel := context.WithTimeout(
				ctx, 6*time.Second,
			)
			defer cancel()

			cmd := exec.CommandContext(
				runCtx,
				"docker", "run",
				"--rm",
				"--pull=never",
				"-i",
				"--network=none",
				"--read-only",
				"--cap-drop=ALL",
				"--security-opt=no-new-privileges",
				"--pids-limit=32",
				"--memory=128m",
				"--cpus=0.5",
				"--user=65534:65534",
				"--tmpfs=/tmp:rw,nosuid,size=16m",
				"--workdir=/tmp",
				"python:3.12-alpine",
				"python", "-I", "-B", "-",
			)

			// 将代码作为标准输入传递，
			// 不经过 shell 拼接。
			cmd.Stdin = strings.NewReader(code)

			output := &boundedWriter{limit: 8192}
			cmd.Stdout = output
			cmd.Stderr = output

			err := cmd.Run()

			if runCtx.Err() != nil {
				return nil, fmt.Errorf("代码执行超时或取消: %w", runCtx.Err())
			}

			if err != nil {
				return nil, fmt.Errorf(
					"代码执行失败: %v; 输出: %s",
					err, output.String(),
				)
			}

			return &CodeOutput{
				Output: output.String(),
			}, nil
		},
	)
}
