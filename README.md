# PR Reviewer Assignment Service

Сервис назначения ревьюверов для Pull Request'ов.

HTTP API описан в `openapi.yaml` в корне проекта.

---

## Что умеет сервис

По HTTP можно:

- создать команду с участниками;
- менять активность пользователя (`is_active`);
- создавать PR и автоматически назначать до двух ревьюверов из команды автора;
- переназначать одного ревьювера на другого;
- получать список PR'ов, где пользователь назначен ревьювером;
- помечать PR как `MERGED` (операция идемпотентная);
- (дополнительно) получить простую статистику по назначениям ревьюверов;
- (дополнительно) массово деактивировать пользователей команды с безопасной переназначаемостью открытых PR.

После перевода PR в статус `MERGED` список ревьюверов менять нельзя.

---

## Архитектура и структура

```text
.
├── cmd/
│   └── main.go              # точка входа: подключение БД, миграции, сервисы, HTTP
├── internal/
│   ├── domain/              # доменные сущности, интерфейсы репозиториев, ошибки
│   ├── service/             # бизнес-логика (TeamService, UserService, PRService)
│   ├── repository/
│   │   └── postgres/        # реализация репозиториев на PostgreSQL
│   ├── http/                # HTTP-роутер, хендлеры, DTO, маппинг ошибок в HTTP
│   └── migrations/          # SQL-миграции и код их запуска
├── docker-compose.yml
├── Dockerfile
├── Makefile
├── golangci.yml
└── openapi.yaml
```

Слои:
- **domain** - чистые модели (`User`, `Team`, `PullRequest`), интерфейсы репозиториев, доменные ошибки
- **service** - доменные сервисы: вся логика назначения ревьюверов, проверка статусов, выбор кандидатов и т.п.
- **repository/postgres** - работа с PostgreSQL, никаких бизнес-правил, только сохранение/чтение
- **http** - JSON запрос/ответ, роутер, маппинг доменных ошибок в нужные HTTP-коды и `ErrorResponse`
- **migrations** - SQL-схема и запуск миграций при старте



## Запуск

### Через Docker Compose

```bash
docker compose up --build
```

Поднимается:

- контейнер с PostgreSQL,
- контейнер с приложением

сервис слушает на `http://localhost:8080`

Быстрая проверка:

```bash
curl -i http://localhost:8080/health
```

Дальше:

```bash
make build   # собрать бинарник в bin/pr-reviewer-service
make run     # или просто go run ./cmd
```

---

## Makefile

команды Makefile:

```bash
make build         # go build -o bin/pr-reviewer-service ./cmd
make run           # go run ./cmd
make test          # go test ./...
make lint          # golangci-lint run ./...
make docker-build  # docker compose build
make docker-up     # docker compose up
make docker-down   # docker compose down
```

---

## Примеры запросов curl

### Создать команду

```bash
curl -i -X POST http://localhost:8080/team/add   -H "Content-Type: application/json"   -d '{
    "team_name": "backend",
    "members": [
      { "user_id": "u1", "username": "Alice",   "is_active": true  },
      { "user_id": "u2", "username": "Bob",     "is_active": true  },
      { "user_id": "u3", "username": "Charlie", "is_active": true  },
      { "user_id": "u4", "username": "Diana",   "is_active": true  }
    ]
  }'
```

### Создать PR

```bash
curl -i -X POST http://localhost:8080/pullRequest/create   -H "Content-Type: application/json"   -d '{
    "pull_request_id":   "pr-1001",
    "pull_request_name": "Add search",
    "author_id":         "u1"
  }'
```

### Переназначить ревьювера

```bash
curl -i -X POST http://localhost:8080/pullRequest/reassign   -H "Content-Type: application/json"   -d '{
    "pull_request_id": "pr-1001",
    "old_user_id":     "u2"
  }'
```

### Merge PR (идемпотентно)

```bash
curl -i -X POST http://localhost:8080/pullRequest/merge   -H "Content-Type: application/json"   -d '{
    "pull_request_id": "pr-1001"
  }'
```

### Получить PR'ы, где пользователь - ревьювер

```bash
curl -i "http://localhost:8080/users/getReview?user_id=u2"
```

### Статистика по назначениям ревьюверов

```bash
curl -i http://localhost:8080/stats/assignments
```


---

## Тесты

- юнит-тесты доменных сервисов: `internal/service/*_test.go`;
- интеграционный HTTP-тест с in-memory репозиториями:
  `internal/http/http_test/http_integration_test.go`.

Запуск:

```bash
make test
# или
go test ./...
```

Интеграционный тест поднимает `httptest.Server`, пробегается по основному флоу:
создание команды -> создание PR -> проверка `/users/getReview` -> merge -> reassign и т.д.

---

## Линтер

Используется `golangci-lint`, конфигурация лежит в `golangci.yml`

Основные включённые линтеры:

- `govet`, `staticcheck`, `gosimple` - поиск подозрительных конструкций;
- `errcheck` - проверка, что ошибки не игнорируются;
- `gofmt`, `goimports` - форматирование
- `ineffassign`, `unused` - неиспользуемые присваивания/символы
- `misspell` - орфография

Запуск:

```bash
make lint
# или
golangci-lint run ./...
```



---

## логика решений некоторых моментов

**Хранение ревьюверов.**  
   Вместо массива user_id в `pull_requests` я сделал отдельную таблицу `pull_request_reviewers`.  
   Так проще считать статистику, удобно индексировать и не лезть в массивы в SQL.


**Тестирование.**  
    Для интеграционного HTTP-теста я использую in-memory реализации репозиториев. Так можно не поднимать отдельную тестовую БД, но при этом пройти основную бизнес-логику полностью через HTTP. 
    Также его я вынес в отдельный пакет `http_test`. Я хочу тестировать HTTP-слой как «чёрный ящик» - через `httptest.Server`, только через публичный конструктор `NewHandler`, без доступа к внутренним деталям пакета `http`.
