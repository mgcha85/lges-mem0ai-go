# mem0-go

> Python [mem0ai](https://github.com/mem0ai/mem0) 라이브러리의 Go 포팅 — 멀티턴 Long-term Memory 구현

## 📖 프로젝트 개요

`mem0-go`는 AI 어시스턴트에 지능형 메모리 레이어를 추가하는 라이브러리입니다. 사용자와의 대화에서 중요한 사실(이름, 직업, 취미 등)을 자동으로 추출하여 장기 기억으로 저장하고, 이후 대화에서 이를 활용하여 개인화된 응답을 제공합니다.

### 핵심 기능

- **자동 Fact 추출**: LLM을 활용하여 대화에서 핵심 정보를 자동 추출
- **지능형 메모리 관리**: 새로운 정보가 들어오면 ADD / UPDATE / DELETE 자동 판단
- **벡터 유사도 검색**: 질문과 관련된 기억을 임베딩 기반으로 검색
- **변경 이력 관리**: 모든 메모리 변경 이력을 SQLite에 기록

## 🛠️ 기술 스택

| 구성요소 | 기술 |
|----------|------|
| 언어 | Go 1.24+ |
| LLM | OpenAI GPT-4.1-nano (gpt-4.1-nano-2025-04-14) |
| 임베딩 | OpenAI text-embedding-3-small |
| 벡터 DB | Qdrant (gRPC) |
| 히스토리 DB | SQLite |
| OpenAI SDK | github.com/sashabaranov/go-openai |
| Qdrant SDK | github.com/qdrant/go-client |

## 📁 소스 트리

```
mem0-go/
├── cmd/chat/main.go              # CLI 챗 애플리케이션
├── pkg/
│   ├── config/config.go           # 설정 (모델, DB, Qdrant 등)
│   ├── prompts/
│   │   ├── prompts.go             # LLM 프롬프트 (fact 추출, 메모리 업데이트)
│   │   └── prompts_test.go
│   ├── llm/
│   │   ├── llm.go                 # LLM 인터페이스
│   │   └── openai.go              # OpenAI 구현
│   ├── embeddings/
│   │   ├── embeddings.go          # Embedder 인터페이스
│   │   └── openai.go              # OpenAI 구현
│   ├── vectorstore/
│   │   ├── store.go               # VectorStore 인터페이스
│   │   └── qdrant.go              # Qdrant gRPC 구현
│   ├── store/
│   │   ├── sqlite.go              # SQLite 히스토리 매니저
│   │   └── sqlite_test.go
│   └── memory/
│       ├── memory.go              # 핵심 Memory 타입 (Add/Search/Get/GetAll/Delete/Reset)
│       └── memory_test.go         # Mock 기반 테스트 (22개)
├── docs/
│   ├── usage.md                   # 사용법
│   ├── development.md             # 개발 문서
│   └── architecture.md            # 아키텍처 문서
├── README.md
├── go.mod
└── go.sum
```

## 🚀 사용법

### 사전 요구사항

- Go 1.24+
- [Qdrant](https://qdrant.tech/) 실행 (Docker 권장)
- OpenAI API 키

### 1. Qdrant 실행

```bash
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

### 2. 환경변수 설정

```bash
export OPENAI_API_KEY="sk-your-api-key"
```

### 3. CLI 챗 실행

```bash
cd mem0-go
go run cmd/chat/main.go
```

### 4. 사용 예시

```
Chat with AI (type 'exit' to quit)
You: 안녕하세요, 제 이름은 민수이고 소프트웨어 엔지니어입니다.
AI: 안녕하세요 민수님! 소프트웨어 엔지니어시군요. 어떤 분야에서 일하시나요?
You: 주로 Go와 Python으로 백엔드 개발을 합니다.
AI: Go와 Python 백엔드 개발을 하시는군요! ...
You: 제 이름이 뭐였죠?
AI: 민수님이시죠! 소프트웨어 엔지니어로 Go와 Python 백엔드 개발을 하시는 것으로 알고 있습니다.
```

### 5. 라이브러리로 사용

```go
package main

import (
    "fmt"
    "github.com/mem0ai/mem0-go/pkg/config"
    "github.com/mem0ai/mem0-go/pkg/memory"
)

func main() {
    cfg := config.DefaultConfig()
    mem, _ := memory.New(cfg)
    defer mem.Close()

    // 메모리 추가
    mem.Add([]memory.Message{
        {Role: "user", Content: "제 이름은 민수입니다"},
        {Role: "assistant", Content: "반갑습니다 민수님!"},
    }, "user1")

    // 메모리 검색
    results, _ := mem.Search("이름이 뭐야?", "user1", 3)
    for _, r := range results.Results {
        fmt.Printf("- %s (score: %.2f)\n", r.Memory, r.Score)
    }
}
```

## ✅ 테스트

```bash
# 전체 단위 테스트 실행 (API 키 불필요)
go test ./pkg/... -v -count=1
```

## 📚 문서

- [사용법](docs/usage.md) — 설치, 설정, 실행 방법
- [개발 문서](docs/development.md) — 개발 과정 및 구현 상세
- [아키텍처 문서](docs/architecture.md) — 시스템 구조 및 로직 설명

## ⚖️ 라이센스

Apache 2.0 — 원본 [mem0ai](https://github.com/mem0ai/mem0)와 동일
