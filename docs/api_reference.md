# API Reference

> lges-mem0ai-go REST API 엔드포인트 문서

## Base URL

```
http://localhost:8080
```

---

## 헬스 체크

### `GET /health`

서버 상태 확인.

**Response** `200 OK`
```json
{
  "status": "ok",
  "version": "2.0.0"
}
```

---

## 사용자 관리

### `GET /users`

전체 사용자 목록 조회.

**Response** `200 OK`
```json
[
  {
    "employee_id": "test001",
    "name": "민규",
    "position": "엔지니어",
    "created_at": "2026-02-15T10:29:54+09:00",
    "updated_at": "2026-02-15T10:29:54+09:00"
  }
]
```

### `GET /users/{employee_id}`

특정 사용자 정보 및 세션 목록 조회.

**Response** `200 OK`
```json
{
  "user": {
    "employee_id": "test001",
    "name": "민규",
    "position": "엔지니어"
  },
  "sessions": [
    {
      "session_id": "sess001",
      "employee_id": "test001",
      "created_at": "2026-02-15T10:29:54+09:00",
      "last_activity": "2026-02-15T10:30:31+09:00"
    }
  ]
}
```

---

## 세션 관리

### `GET /sessions/{session_id}`

특정 세션 정보 조회.

**Response** `200 OK`
```json
{
  "session_id": "sess001",
  "employee_id": "test001",
  "created_at": "2026-02-15T10:29:54+09:00",
  "last_activity": "2026-02-15T10:30:31+09:00"
}
```

---

## 메모리 관리

### `POST /memory`

메시지에서 핵심 정보(Fact)를 추출하여 메모리에 저장.

**Request Body**
```json
{
  "employee_id": "test001",
  "session_id": "sess001",
  "messages": [
    {"role": "user", "content": "저는 Python과 Go를 주로 사용합니다."}
  ]
}
```

**Response** `200 OK`
```json
{
  "message": "Memory added successfully"
}
```

### `GET /memory/{employee_id}/{session_id}`

특정 세션의 전체 메모리 조회.

**Response** `200 OK`
```json
[
  {
    "id": "0704ffbc-a2ff-417a-b84e-9c2dc601a069",
    "memory": "이름은 민규 소프트웨어 엔지니어 Go와 Python을 주로 사용"
  }
]
```

### `POST /memory/search`

쿼리와 관련된 메모리를 벡터 유사도 기반으로 검색.

**Request Body**
```json
{
  "employee_id": "test001",
  "session_id": "sess001",
  "query": "프로그래밍 언어",
  "limit": 5
}
```

**Response** `200 OK`
```json
{
  "results": [
    {
      "id": "0704ffbc-a2ff-417a-b84e-9c2dc601a069",
      "memory": "이름은 민규 소프트웨어 엔지니어 Go와 Python을 주로 사용",
      "score": 0.84877694
    }
  ]
}
```

### `DELETE /memory/{employee_id}/{session_id}`

특정 세션의 전체 메모리 삭제.

**Response** `200 OK`
```json
{
  "message": "All memories deleted for session sess001"
}
```

### `DELETE /users/{employee_id}/memories`

특정 사용자의 모든 세션 메모리 삭제.

**Response** `200 OK`
```json
{
  "message": "Deleted memories from 1 sessions for user test001"
}
```

---

## 채팅 (메모리 기반)

### `POST /chat`

메모리를 활용한 개인화된 AI 응답 생성. 자동으로 관련 메모리를 검색하고, 현재 대화 내용을 메모리에 추가.

**Request Body**
```json
{
  "employee_id": "test001",
  "session_id": "sess001",
  "message": "제가 어떤 프로그래밍 언어를 사용하는지 알고 있나요?",
  "user_name": "민규",
  "user_position": "엔지니어"
}
```

**Response** `200 OK`
```json
{
  "response": "네, 민규 님. 당신은 주로 Go와 Python 프로그래밍 언어를 사용하는 소프트웨어 엔지니어입니다.",
  "memories_used": [
    {
      "id": "0704ffbc-a2ff-417a-b84e-9c2dc601a069",
      "memory": "이름은 민규 소프트웨어 엔지니어 Go와 Python을 주로 사용",
      "score": 0.86571443
    }
  ],
  "user_info": {
    "employee_id": "test001",
    "name": "민규",
    "position": "엔지니어"
  }
}
```

---

## 에러 응답

모든 에러는 다음 형식으로 반환됩니다:

```json
{
  "detail": "에러 메시지"
}
```

| HTTP Status | 설명 |
|-------------|------|
| 400 | 잘못된 요청 (Request Body 파싱 실패) |
| 404 | 리소스 없음 (사용자/세션 미존재) |
| 500 | 서버 내부 오류 (DB/LLM/벡터 DB 오류) |
