# Raven — Code Style & Conventions

## Go Conventions

### Architecture Pattern
- **3-layer**: Handler → Service → Repository
- Each handler file defines a `*Servicer` interface for its service dependency
- Each service file defines a `*Repository` interface for its data dependency
- Constructor functions: `NewXxxHandler(svc)`, `NewXxxService(repo)`, `NewXxxRepository(pool)`

### Naming
- **Files**: lowercase, one file per domain (e.g., `chat.go`, `chat_test.go`)
- **Structs**: PascalCase (`ChatHandler`, `ChatService`, `ChatRepository`)
- **Interfaces**: `*Servicer` for handler deps, `*Repository` for service deps, `*Client` for external deps
- **Methods**: PascalCase for exported, camelCase for unexported
- **SQL query constants**: `queryXxxVerb` pattern (e.g., `queryChatSessionInsert`, `queryChatMessageList`)
- **Config env vars**: `RAVEN_*` prefix

### Code Style
- **Linter**: golangci-lint v2 with govet, errcheck, staticcheck, ineffassign, revive
- **Revive rules**: exported, blank-imports, context-as-argument, error-return, error-strings, increment-decrement, var-naming
- Standard Go formatting (gofmt)
- No AI attribution in commits (no Co-Authored-By trailers)

### Testing
- `testify` for assertions (`assert`, `require`)
- `testcontainers-go` for integration tests (pgvector/pgvector:pg17 image)
- `alicebob/miniredis` for Redis/Valkey mocking
- Test files co-located with source (`*_test.go`)
- Integration tests in `internal/integration/`
- Table-driven tests where appropriate

### Error Handling
- `pkg/apierror.AppError` for API error responses (Code, Message, Detail)
- Handlers return JSON error responses via `c.AbortWithStatusJSON`
- Repository layer uses pgx error checking (e.g., `isNoRows`)

### Documentation
- Swagger annotations on handler methods (`@Summary`, `@Tags`, `@Param`, `@Success`, `@Failure`, `@Router`)
- Go doc comments on exported types and functions

## Python Conventions (ai-worker)

- **Linter**: ruff (line-length 100, target py312)
- **Ruff rules**: E, F, I, N, W, UP, B, A, SIM
- **Type checking**: mypy (python 3.12, ignore_missing_imports)
- **Testing**: pytest with asyncio_mode="auto", coverage fail_under=70%
- **Generated code**: excluded from lint/mypy/coverage (`raven_worker/generated/`)
- **Structured logging**: structlog

## Frontend Conventions (Vue.js)

- **TypeScript strict** with vue-tsc
- **ESLint**: @eslint/js + typescript-eslint + eslint-plugin-vue (flat/recommended) + prettier
- **Formatting**: Prettier
- **State management**: Pinia stores (one per domain, co-located .spec.ts)
- **API layer**: dedicated api/ modules per domain
- **Composables**: useAuth, useFeatureFlag, useCookieConsent, useMediaQuery
- **CSS**: Tailwind CSS v4

## Branch Naming
`type/descriptor` format: `feat/`, `fix/`, `refactor/`, `ci/`, `chore/`, `deps/`

## PR Workflow
- Never push directly to main
- Squash merge only
- Always `gh pr merge <N> --auto --squash` immediately after PR creation
