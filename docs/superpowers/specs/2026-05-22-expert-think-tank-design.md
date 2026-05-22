# Expert Think Tank — 自生长专家智囊团

**日期**: 2026-05-22
**状态**: 设计稿 v1

## 概述

在主 Agent 执行过程中，自动从经验中沉淀专家 Agent，形成一个自生长的"专家智囊团"。主 Agent 负责编排调度，专家团队随着系统运行不断进化。

## 核心哲学

- **不是预制，而是自生长** — 专家不是手动配置的，而是系统从执行经验中提炼的
- **能力不是配置出来的，是生长出来的**
- **信息密度 > 上下文长度** — 严格控制注入量，只加载最相关的专家

## 架构总览

```
                  ┌─────────────────────────────────────┐
                  │           Main Agent                 │
                  │   (干活 + 自主调用专家工具)             │
                  └──────────┬──────────────────────────┘
                             │ 执行日志
                             ▼
                  ┌──────────────────┐
                  │ Pattern Analyzer  │ ←── 定时复盘 / 触发式复盘
                  │ (提取模式/沉淀)    │
                  └──────────┬───────┘
                             │ 创建/更新专家
                             ▼
                  ┌──────────────────┐
                  │  Expert Registry  │ ←── 调度器查询
                  │ (存储/索引/统计)   │
                  └──────────┬───────┘
                             │ 匹配结果
                             ▼
                  ┌──────────────────┐
                  │   Dispatcher      │
                  │ (咨询/代理路由)    │
                  └──────────┬───────┘
                             │ 注入/委派
                             ▼
                  ┌──────────────────┐
                  │    Main Agent     │──→ 继续执行
                  └──────────────────┘
```

## 模块一：Pattern Analyzer（模式分析器）

### 职责
从执行日志中识别重复任务模式，自动沉淀为专家 Agent。

### 触发时机
1. **定期复盘** — 每 N 次任务后 / 系统空闲时，复用现有的 reflect 机制
2. **触发式** — 某类任务频率突增 / 某工具被异常高频调用时立即触发

### 工作流程
1. 从 L3/L4 归档日志中提取任务片段（任务描述 + 工具链 + 耗时 + 结果）
2. 识别相似片段聚类（如"修改 Go 代码 → 跑测试 → 修复"出现 ≥ 5 次）
3. 评估聚类质量（频率、工具链固定程度、任务结果质量）
4. 生成专家草案（LLM 自动提炼共享 prompt + 提取公共工具集）
5. 存入 Expert Registry

### 产出物示例
```yaml
name: go-code-reviewer
summary: Go 代码审查与测试修复
trigger_pattern: "修改 .go 文件 → go test → 修复编译错误"
tools: [file_read, file_patch, code_run]
prompt: "You are a Go code review specialist..."
frequency: 12
confidence: 0.85
```

## 模块二：Expert Registry（专家注册表）

### 存储结构
```
data/experts/
├── go-code-reviewer/
│   ├── expert.yaml      # 元信息 + 工具白名单 + 触发模式
│   ├── prompt.md        # 专家 system prompt
│   └── stats.jsonl      # 调用记录（时间、结果、评分）
├── shell-pro/
│   └── ...
└── registry.index       # 全局索引（FTS5），用于快速匹配
```

### 接口

| 操作 | 触发者 | 说明 |
|------|--------|------|
| `ListExperts()` | 主 Agent、Dispatcher | 列出所有可用专家 |
| `GetExpert(name)` | 主 Agent、Dispatcher | 获取专家详情 |
| `CreateExpert(spec)` | Pattern Analyzer | 创建新专家 |
| `UpdateExpert(name, spec)` | Pattern Analyzer | 优化已有专家 |
| `DeleteExpert(name)` | Pattern Analyzer / 主 Agent | 废弃低效专家 |
| `RecordCall(name, result)` | Dispatcher | 记录调用结果 |
| `SearchExperts(task_desc)` | Dispatcher | 根据任务描述匹配最相关专家 |

### 专家生命周期
```
草案（Pattern Analyzer 发现）
  → 活跃（首次被 Dispatcher 加载使用）
  → 成熟（多次调用，prompt 被持续优化）
  → 归档（长期未用 / 低效 / 被新专家替代）
```

## 模块三：Dispatcher（运行时调度器）

### 职责
在每个任务开始时，匹配最相关专家并决定调用模式。

### 工作流程
```
主 Agent 收到新任务
        │
        ▼
Dispatcher 分析任务描述
        │
        ├── 匹配到专家 → 判断调用模式
        │       ├── 咨询模式：注入专家 prompt + 合并工具白名单
        │       └── 代理模式：独立 LLM 调用执行，结果返回主 Agent
        │
        └── 未匹配到专家 → 正常执行，日志留给 Pattern Analyzer
```

### 匹配策略
- Phase 1：基于 `trigger_pattern` 的精确子串匹配 + 专家名称/描述的关键词匹配
- Phase 2：升级为 FTS5 全文索引
- Phase 3：引入调用统计权重排序
- 每次最多加载 2-3 个最相关专家，默认只加载 1 个

### 两种调用模式
- **咨询模式（Consult）**：专家 prompt 注入到主 Agent instruction，工具白名单合并到主 Agent 工具集
- **代理模式（Delegate）**：启动独立 LLM 调用，使用专家的 prompt + 工具集独立执行，结果结构化返回

## 完整闭环
```
执行 → 日志 → 复盘 → 提炼 → 注册 → 匹配 → 注入/委派 → 执行
                                                          │
                                                          └── 持续优化循环
```

## 与现有系统的集成

- **复用 reflect 机制** — Pattern Analyzer 的定时复盘挂载到现有的 reflect/idle-watch 模式
- **复用 memory 系统** — 执行日志从 L3/L4 读取，专家沉淀产物写入 L2 记忆
- **复用 model provider** — 代理模式下的独立 LLM 调用复用现有的 model factory
- **复用 ADK 单 Agent 架构** — 咨询模式不改变 ADK 架构；代理模式通过工具内部启动子 LLM 调用

## 阶段规划建议

### Phase 1（MVP）
- Expert Registry 存储 + CRUD 接口
- Dispatcher 的咨询模式
- 手工创建第一个专家验证链路

### Phase 2
- Pattern Analyzer 基础版（基于工具链序列精确匹配的历史任务聚类 + LLM 提炼）
- Dispatcher 的代理模式
- 完整的生命周期管理

### Phase 3
- FTS5 索引优化
- 调用统计驱动的自动优化
- 专家版本管理