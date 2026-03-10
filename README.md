# lges-mem0ai-go

> Python `lge-mem0ai` 프로젝트의 Go 포팅 — 멀티턴 Long-term Memory 백엔드 서버

## 📖 프로젝트 개요

`lges-mem0ai-go`는 AI 어시스턴트에 지능형 메모리 레이어를 제공하는 REST API 서버입니다. `mem0-go` 라이브러리를 기반으로 사용자별 세션 격리된 장기 기억을 관리하며, Python 버전과 동일한 API를 제공합니다.

### 핵심 기능

- **세션 기반 메모리 관리**: 사용자별, 세션별로 격리된 메모리 저장소
- **자동 Fact 추출**: LLM을 활용한 대화 내 핵심 정보 자동 추출
- **벡터 유사도 검색**: 질문과 관련된 기억을 임베딩 기반으로 검색
- **메모리 기반 채팅**: 축적된 기억을 활용한 개인화된 AI 응답
- **사용자/세션 관리**: SQLite 기반 사용자 및 세션 자동 생성/관리

## 🛠️ 기술 스택

| 구성요소 | 기술 |
|----------|------|
| 언어 | Go 1.24+ |
| HTTP 서버 | net/http (표준 라이브러리) |
| LLM | DeepSeek / OpenAI 호환 |
| 임베딩 | OpenAI text-embedding-3-small / ONNX |
| 벡터 DB | Qdrant (gRPC) |
| 사용자 DB | SQLite (WAL 모드) |
| 메모리 엔진 | mem0-go (로컬 의존성) |

## 📁 소스 트리

```
lges-mem0ai-go/
├── cmd/server/main.go               # HTTP 서버 엔트리포인트
├── internal/
│   ├── config/config.go             # .env 설정 로드
│   ├── database/database.go         # SQLite 사용자/세션 관리
│   ├── models/models.go             # Request/Response 구조체
│   ├── service/service.go           # mem0-go Memory 래핑 서비스
│   └── handler/handler.go           # REST API 핸들러
├── start.sh                         # 서버 기동 스크립트
├── .env                             # 환경변수 파일
├── .gitignore
├── go.mod
└── README.md
```

## 🚀 사용법

### 사전 요구사항

- Go 1.24+
- [Qdrant](https://qdrant.tech/) 실행 (Docker 권장)
- OpenAI / DeepSeek API 키

### 1. Qdrant 실행

```bash
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

### 2. 환경변수 설정

`.env` 파일을 수정합니다:

```env
OPENAI_API_KEY=your-api-key
OPENAI_API_BASE=https://api.deepseek.com/v1
OPENAI_MODEL=deepseek-chat
EMBEDDING_PROVIDER=openai
EMBEDDING_MODEL=text-embedding-3-small
QDRANT_HOST=localhost
QDRANT_PORT=6334
SERVER_PORT=8080
```

### 3. 서버 실행

```bash
chmod +x start.sh
./start.sh
```

또는 직접 실행:

```bash
go run cmd/server/main.go
```

## 📡 API 엔드포인트

### 헬스 체크

```bash
curl http://localhost:8080/health
# {"status":"ok","version":"2.0.0"}
```

### 채팅 (메모리 기반)

```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{
    "employee_id": "emp001",
    "session_id": "sess001",
    "message": "안녕하세요, 제 이름은 민규이고 소프트웨어 엔지니어입니다.",
    "user_name": "민규",
    "user_position": "엔지니어"
  }'
```

### 메모리 추가

```bash
curl -X POST http://localhost:8080/memory \
  -H "Content-Type: application/json" \
  -d '{
    "employee_id": "emp001",
    "session_id": "sess001",
    "messages": [
      {"role": "user", "content": "저는 Python과 Go를 주로 사용합니다."}
    ]
  }'
```

### 메모리 검색

```bash
curl -X POST http://localhost:8080/memory/search \
  -H "Content-Type: application/json" \
  -d '{
    "employee_id": "emp001",
    "session_id": "sess001",
    "query": "프로그래밍 언어",
    "limit": 5
  }'
```

### 세션 메모리 조회

```bash
curl http://localhost:8080/memory/emp001/sess001
```

### 사용자 목록 조회

```bash
curl http://localhost:8080/users
```

### 세션 메모리 삭제

```bash
curl -X DELETE http://localhost:8080/memory/emp001/sess001
```

## ✅ 빌드 & 테스트

```bash
# 빌드 확인
go build ./...

# 정적 분석
go vet ./...
```

## 📊 Python 버전과의 차이점

| 항목 | Python | Go |
|------|--------|-----|
| 웹 프레임워크 | FastAPI | net/http |
| LLM 라이브러리 | openai-python | go-openai |
| 메모리 엔진 | mem0 (Python) | mem0-go |
| 설정 관리 | pydantic-settings | 내장 .env 파서 |
| 의존성 주입 | FastAPI Depends | 생성자 패턴 |

## ⚖️ 라이센스

Apache 2.0
