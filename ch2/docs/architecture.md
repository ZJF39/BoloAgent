# BoloAgent 架构

BoloAgent 是一个使用 Go 和 Eino SDK 构建的 Agent 学习项目。

项目采用分阶段学习方式。

Stage 1 的主要目标是手工实现最小 Agent Loop，包括模型调用、Tool Calling、
工具执行、Tool Result 返回、最大执行步数、超时控制以及错误处理。

Agent Loop 的核心流程是：

Observe → Think → Act → Observe。

模型负责根据当前上下文决定下一步动作，
Go 程序负责真正执行工具，并把工具执行结果重新作为 Observation 返回给模型。

BoloAgent 当前的最小 Agent Loop 示例设置最大执行步数为 5。
如果超过最大步数仍然没有得到最终回答，则主动终止 Agent。

整个 Agent 的运行还通过 context.WithTimeout 设置全局超时。
当前学习示例中的全局超时时间为 30 秒。

Stage 2 开始学习 RAG、Memory 和更复杂的 Tool Use。