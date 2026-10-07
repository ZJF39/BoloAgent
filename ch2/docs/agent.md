# Agent 与 RAG

Agent 的核心特点是模型拥有一定程度的决策权。

普通 Workflow 的执行路径通常由程序提前确定，
而 Agent 会根据 Observation 动态决定下一步 Action。

Tool Call 并不意味着工具已经执行。

模型只会产生一个 Tool Call 请求，其中包括工具名称和参数。
真正的工具执行由 Go 程序完成。

工具执行完成以后，程序需要构造 ToolMessage，
并通过 ToolCallID 将工具执行结果和之前的 Tool Call 对应起来。

RAG 是 Retrieval-Augmented Generation，即检索增强生成。

RAG 通常包括四个主要步骤：

1. Chunk：将长文档切成较小的文本块。
2. Embed：将文本块转换成向量。
3. Retrieve：根据用户问题检索最相关的文本块。
4. Generate：把检索结果作为上下文交给大语言模型生成答案。

最基础的 RAG 本身通常是 Workflow，而不是 Agent，
因为检索和生成的执行流程是开发者提前规定好的。