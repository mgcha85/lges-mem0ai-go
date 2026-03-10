# 개발 문서

## 개발 배경

Python `mem0ai` 라이브러리의 핵심 멀티턴 메모리 기능을 Go로 포팅하는 프로젝트입니다. 원본의 `Memory` 클래스가 제공하는 `Add`, `Search`, `Get`, `GetAll`, `Delete`, `Reset` 기능을 동일하게 구현하되, Go의 인터페이스 기반 설계를 활용하여 테스트 용이성을 확보했습니다.

## 구현 상세

### 포팅한 핵심 기능

| Python | Go | 설명 |
|--------|----|------|
| `Memory.add()` | `Memory.Add()` | 메시지에서 fact 추출 → 기존 메모리 비교 → ADD/UPDATE/DELETE |
| `Memory.search()` | `Memory.Search()` | 쿼리 임베딩 → 벡터 유사도 검색 |
| `Memory.get()` | `Memory.Get()` | ID로 단일 메모리 조회 |
| `Memory.get_all()` | `Memory.GetAll()` | 사용자별 전체 메모리 조회 |
| `Memory.delete()` | `Memory.Delete()` | ID로 메모리 삭제 |
| `Memory.reset()` | `Memory.Reset()` | 전체 초기화 |
| `Memory.history()` | `Memory.History()` | 변경 이력 조회 |

### 인터페이스 설계

Go의 인터페이스를 활용하여 각 컴포넌트를 교체 가능하게 설계했습니다:

```go
// LLM 인터페이스
type LLM interface {
    GenerateResponse(messages []Message, jsonMode bool) (string, error)
}

// Embedder 인터페이스
type Embedder interface {
    Embed(text string) ([]float32, error)
}

// VectorStore 인터페이스
type VectorStore interface {
    Insert(vectors [][]float32, ids []string, payloads []map[string]interface{}) error
    Search(query string, vector []float32, limit int, filters map[string]string) ([]SearchResult, error)
    Get(id string) (*VectorRecord, error)
    List(filters map[string]string, limit int) ([]VectorRecord, error)
    Update(id string, vector []float32, payload map[string]interface{}) error
    Delete(id string) error
    Reset() error
}
```

이 설계 덕분에 테스트 시 Mock 객체를 주입할 수 있습니다:

```go
mem := memory.NewWithDeps(mockLLM, mockEmbedder, mockVectorStore, sqliteDB)
```

### ONNX 임베딩 (최적화)

비용 절감과 속도 향상을 위해 로컬 ONNX 런타임 지원을 추가했습니다:

- **Tokenizer**: `pkg/tokenizer`에 Go 기반 WordPiece 토크나이저 직접 구현 (외부 의존성 최소화)
- **Runtime**: `github.com/yalue/onnxruntime_go` 활용
- **Pipeline**: Tokenization → Inference → Mean Pooling → Normalization
- **설정**: `EMBEDDING_PROVIDER="onnx"` 환경변수로 전환 가능

- **UserMemoryExtractionPrompt**: 사용자 메시지에서 fact를 추출하는 프롬프트
- **DefaultUpdateMemoryPrompt**: 기존 메모리와 새 fact를 비교하여 ADD/UPDATE/DELETE/NONE을 결정하는 프롬프트
- **GetUpdateMemoryMessages()**: 기존 메모리와 새 fact를 결합한 최종 프롬프트 생성

### UUID 매핑 기법

Python 원본과 동일하게, LLM이 UUID를 정확하게 다루지 못하는 문제를 해결하기 위해 정수 ID로 매핑합니다:

```
실제 UUID: "a1b2c3d4-..." → LLM용 ID: "0"
실제 UUID: "e5f6g7h8-..." → LLM용 ID: "1"
```

LLM 응답을 받은 후 다시 실제 UUID로 변환하여 벡터 스토어 작업을 수행합니다.

## 테스트 전략

### Mock 기반 단위 테스트

API 키 없이 전체 로직을 검증할 수 있도록 Mock 구현체를 작성했습니다:

- **MockLLM**: 미리 정의된 JSON 응답을 순서대로 반환
- **MockEmbedder**: 텍스트 해시 기반의 결정적 벡터 생성
- **MockVectorStore**: Go map 기반 인메모리 벡터 저장소

### 테스트 커버리지

| 패키지 | 테스트 수 | 커버 범위 |
|--------|-----------|-----------|
| `pkg/memory` | 22개 | Add(새로운/업데이트/삭제/빈 fact), Search, Get, GetAll, Delete, Reset, History, 멀티턴, 프롬프트 검증, JSON 직렬화, 유틸리티 함수 |
| `pkg/prompts` | 5개 | 날짜 포함, 키 디렉티브, 액션 타입, 빈 메모리, 기존 메모리 |
| `pkg/store` | 7개 | CRUD, 리셋, 미존재 조회, 다중 메모리 격리 |

### 실행 방법

```bash
# 전체 테스트
go test ./pkg/... -v -count=1

# 특정 테스트만
go test ./pkg/memory/ -v -run TestMultiTurnConversation -count=1
```
