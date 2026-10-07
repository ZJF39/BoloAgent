package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"

	"BoloAgent/ch2/rag"
)

func main() {
	// 整个 RAG 请求最多运行 60 秒。
	ctx, cancel := context.WithTimeout(
		context.Background(),
		60*time.Second,
	)
	defer cancel()

	// ========================================
	// 1. Load
	// Markdown → Document
	// ========================================

	documents, err :=
		rag.LoadMarkdownDocuments("./docs")

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"加载原始文档: %d 个\n",
		len(documents),
	)

	// ========================================
	// 2. Chunk
	// Document → Chunks
	// ========================================

	chunks, err :=
		rag.SplitDocuments(
			ctx,
			documents,
			300, // ChunkSize
			50,  // OverlapSize
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"切分后 Chunk: %d 个\n",
		len(chunks),
	)

	for _, chunk := range chunks {
		fmt.Printf(
			"- %s (%d 字符)\n",
			chunk.ID,
			len([]rune(chunk.Content)),
		)
	}

	// ========================================
	// 3. 创建 Embedder
	// ========================================

	embedder, err :=
		rag.NewEmbedder(ctx)

	if err != nil {
		log.Fatal(err)
	}

	// ========================================
	// 4. Embed Documents
	// Chunks → Vectors
	// ========================================

	fmt.Println("\n正在生成 Document Embedding...")

	vectors, err :=
		rag.EmbedDocuments(
			ctx,
			embedder,
			chunks,
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"Embedding 完成: %d 个向量\n",
		len(vectors),
	)

	if len(vectors) > 0 {
		fmt.Printf(
			"向量维度: %d\n",
			len(vectors[0]),
		)
	}

	// ========================================
	// 5. Store
	// Document + Vector → MemoryStore
	// ========================================

	store := rag.NewMemoryStore()

	if err := store.Add(
		chunks,
		vectors,
	); err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"Vector Store 当前记录: %d\n",
		store.Size(),
	)

	// ========================================
	// 6. 用户输入问题
	// ========================================

	fmt.Print("\n请输入问题: ")

	scanner := bufio.NewScanner(os.Stdin)

	if !scanner.Scan() {
		log.Fatal("无法读取用户输入")
	}

	question :=
		strings.TrimSpace(scanner.Text())

	if question == "" {
		log.Fatal("问题不能为空")
	}

	// ========================================
	// 7. Retrieve
	// Query → Embedding → Similarity → TopK
	// ========================================

	results, err :=
		rag.Retrieve(
			ctx,
			embedder,
			store,
			question,
			3,
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n===== 检索结果 =====")

	for i, doc := range results {

		fmt.Printf(
			"\n[%d] source=%v chunk=%v score=%.4f\n",
			i+1,
			doc.MetaData["source"],
			doc.MetaData["chunk"],
			doc.Score(),
		)

		fmt.Println(doc.Content)
	}

	// ========================================
	// 8. 创建 ChatModel
	//
	// 这里继续使用你之前的
	// OpenAI-compatible DeepSeek 配置。
	// ========================================

	apiKey := os.Getenv("CHAT_API_KEY")
	modelName := os.Getenv("CHAT_MODEL")
	baseURL := os.Getenv("CHAT_BASE_URL")

	if apiKey == "" {
		log.Fatal(
			"CHAT_API_KEY 未设置",
		)
	}

	if modelName == "" {
		log.Fatal(
			"CHAT_MODEL 未设置",
		)
	}

	chatModel, err :=
		openai.NewChatModel(
			ctx,
			&openai.ChatModelConfig{
				APIKey:  apiKey,
				Model:   modelName,
				BaseURL: baseURL,
			},
		)

	if err != nil {
		log.Fatal(err)
	}

	// ========================================
	// 9. Build Prompt
	//
	// Question
	// +
	// Retrieved Documents
	// +
	// Citations
	// ========================================

	messages :=
		rag.BuildRAGMessages(
			question,
			results,
		)

	// ========================================
	// 10. Generate
	// ========================================

	response, err :=
		chatModel.Generate(
			ctx,
			messages,
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(
		"\n===== RAG 最终回答 =====",
	)

	fmt.Println(
		response.Content,
	)

	// ========================================
	// 11. 再把 Citation 映射打印出来
	// ========================================

	fmt.Println(
		"\n===== Citation Mapping =====",
	)

	for i, doc := range results {
		fmt.Printf(
			"[%d] %v#chunk-%v\n",
			i+1,
			doc.MetaData["source"],
			doc.MetaData["chunk"],
		)
	}
}
