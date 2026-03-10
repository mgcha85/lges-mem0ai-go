# 사용법

## 사전 요구사항

| 항목 | 설명 |
|------|------|
| Go | 1.24 이상 |
| Qdrant | 벡터 데이터베이스 (Docker 권장) |
| OpenAI API Key | LLM 및 임베딩용 |

## 설치 및 설정

### 1. Qdrant 실행

Docker를 사용하여 Qdrant를 실행합니다:

```bash
docker run -d --name qdrant -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

- **6333**: REST API 포트
- **6334**: gRPC 포트 (mem0-go가 사용)

### 2. 환경변수 설정

```bash
export OPENAI_API_KEY="sk-your-api-key-here"

# (선택) 로컬 ONNX 임베딩 사용 시
export EMBEDDING_PROVIDER="onnx"
# ONNX 모델 다운로드 스크립트 실행 필요 (아래 참조)
```

### 3. 프로젝트 빌드

```bash
cd mem0-go
go build ./...
```

## CLI 챗 실행

```bash
go run cmd/chat/main.go
```

실행하면 대화형 인터페이스가 시작됩니다:

```
Chat with AI (type 'exit' to quit)
You: 안녕하세요! 제 이름은 김철수입니다.
AI: 안녕하세요 김철수님! 반갑습니다. 무엇을 도와드릴까요?
You: 저는 주말마다 등산을 즐깁니다.
AI: 등산을 좋아하시는군요! 좋은 취미를 가지고 계시네요.
You: 제 취미가 뭐였죠?
AI: 주말마다 등산을 즐기신다고 하셨습니다!
You: exit
Goodbye!
```

## 라이브러리 사용법

### Memory 생성

```go
import (
    "github.com/mem0ai/mem0-go/pkg/config"
    "github.com/mem0ai/mem0-go/pkg/memory"
)

cfg := config.DefaultConfig()
mem, err := memory.New(cfg)
if err != nil {
    log.Fatal(err)
}
defer mem.Close()
```

### 메모리 추가 (Add)

대화 메시지를 전달하면 LLM이 자동으로 중요한 사실을 추출합니다:

```go
resp, err := mem.Add([]memory.Message{
    {Role: "user", Content: "제 이름은 민수이고, 30살입니다"},
    {Role: "assistant", Content: "반갑습니다 민수님!"},
}, "user1")

// resp.Results: [{ID: "abc-123", Memory: "이름은 민수", Event: "ADD"}, ...]
```

### 메모리 검색 (Search)

질문과 관련된 메모리를 벡터 유사도로 검색합니다:

```go
results, err := mem.Search("이름이 뭐야?", "user1", 3)
for _, r := range results.Results {
    fmt.Printf("- %s (score: %.2f)\n", r.Memory, r.Score)
}
```

### 전체 메모리 조회 (GetAll)

```go
all, err := mem.GetAll("user1", 100)
for _, r := range all {
    fmt.Printf("[%s] %s\n", r.ID, r.Memory)
}
```

### 단일 메모리 조회 (Get)

```go
result, err := mem.Get("memory-id-here")
```

### 메모리 삭제 (Delete)

```go
err := mem.Delete("memory-id-here")
```

### 전체 초기화 (Reset)

```go
err := mem.Reset()
```

### 변경 이력 조회 (History)

```go
history, err := mem.History("memory-id-here")
for _, h := range history {
    fmt.Printf("[%s] %s → %s\n", h.Event, h.PrevValue, h.NewValue)
}
```

## 설정 커스터마이징

`config.MemoryConfig`를 직접 설정할 수 있습니다:

```go
cfg := config.MemoryConfig{
    LLMModel:       "gpt-4o-mini",
    LLMAPIKey:      os.Getenv("OPENAI_API_KEY"),
    EmbeddingModel: "text-embedding-3-small",
    EmbeddingDims:  1536,
    QdrantHost:     "localhost",
    QdrantPort:     6334,
    CollectionName: "my-memories",
    HistoryDBPath:  "/path/to/history.db",
}

## 로컬 ONNX 임베딩 사용 (비용 절감)

OpenAI 임베딩 대신 로컬 CPU에서 동작하는 ONNX 모델을 사용할 수 있습니다.

### 1. 모델 다운로드 및 설정

제공된 스크립트를 사용하여 모델과 런타임을 다운로드합니다:

```bash
chmod +x scripts/download_onnx.sh
./scripts/download_onnx.sh
```

### 2. 실행 시 환경변수 설정

```bash
export EMBEDDING_PROVIDER="onnx"
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$(pwd)/.mem0-go/lib

go run cmd/chat/main.go
```

이 모드에서는 `text-embedding-3-small` 대신 `all-MiniLM-L6-v2` 모델이 로컬에서 실행됩니다.
```

## 테스트 실행

```bash
# 전체 단위 테스트 (Mock 기반, API 키 불필요)
go test ./pkg/... -v -count=1

# 특정 패키지만 테스트
go test ./pkg/memory/ -v -count=1
go test ./pkg/store/ -v -count=1
go test ./pkg/prompts/ -v -count=1
```
