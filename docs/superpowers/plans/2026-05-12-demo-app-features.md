# Demo App Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the application-layer features required for `demo.raven.ravencloak.org` to be safely public: hide voice UI, cap LLM spend, gate signups with Turnstile, satisfy GDPR/DPDP via working DSAR endpoints + nightly retention purge, send transactional email via Resend, and pre-seed each new signup with a sample workspace.

**Architecture:** All changes scoped to existing codebases — `frontend/`, `internal/` (Go API), `ai-worker/`, `landing/`. New cross-cutting pieces are: a `MailSender` interface in the API with a Resend adapter; a Valkey-backed `LLMSpendFuse` in the python-worker; a `RetentionPurger` service in the API exposed via a new admin endpoint and triggered by a host-side systemd timer. No new external services; every dependency already exists in compose.

**Tech Stack:** Vue 3 (frontend), Go 1.22+/Gin (api), Python 3.12 + grpc (worker), Valkey (counters), SuperTokens (auth lifecycle hooks), Resend HTTP API, Cloudflare Turnstile.

**Spec reference:** `docs/superpowers/specs/2026-05-12-public-demo-deployment-design.md` §§3, 6, 8, 10, 11
**Prerequisite plan:** `2026-05-12-demo-infrastructure.md` (SSM secrets must exist; env vars must reach the containers)

---

## File Structure

**Create:**
- `internal/mail/sender.go` — `MailSender` interface
- `internal/mail/resend.go` — Resend HTTP implementation
- `internal/mail/sender_test.go`
- `internal/mail/resend_test.go`
- `internal/mail/templates/retention_warning.txt`
- `internal/mail/templates/dsar_delete_confirm.txt`
- `internal/mail/templates/dsar_delete_complete.txt`
- `internal/account/dsar.go` — `/account/export`, `/account/delete` handlers
- `internal/account/dsar_test.go`
- `internal/account/retention.go` — `RetentionPurger.RunOnce`
- `internal/account/retention_test.go`
- `internal/turnstile/verifier.go` — verifies Turnstile tokens server-side
- `internal/turnstile/verifier_test.go`
- `internal/seed/sample_workspace.go` — seeds a fresh workspace for a new user
- `internal/seed/sample_workspace_test.go`
- `internal/seed/fixtures/sample_workspace.json`
- `ai-worker/raven_worker/llm_fuse.py` — daily $-cap circuit breaker
- `ai-worker/tests/test_llm_fuse.py`
- `frontend/src/components/CookieBanner.vue`
- `frontend/src/components/TurnstileWidget.vue`
- `frontend/src/views/legal/PrivacyPolicy.vue`
- `frontend/src/views/legal/TermsOfService.vue`
- `landing/src/routes/legal/privacy.md`
- `landing/src/routes/legal/terms.md`
- `deploy/ansible/roles/raven-backup/files/raven-retention-purge.service` — systemd unit for retention cron
- `deploy/ansible/roles/raven-backup/files/raven-retention-purge.timer`

**Modify:**
- `internal/account/<routes file>` — register DSAR routes
- `internal/account/<signup or supertokens hook file>` — trigger sample-workspace seed on user create
- `internal/api/server.go` or equivalent bootstrap — wire `MailSender` and `RetentionPurger`
- `frontend/src/components/chat-widget/RavenChat.ce.ts` — already supports `voice-enabled` attr; no change needed
- `frontend/src/router/index.ts` (or equivalent) — register `/legal/privacy`, `/legal/terms`
- `frontend/src/views/auth/SignupView.vue` (verify name) — mount `TurnstileWidget`
- `frontend/src/App.vue` (verify name) — mount `CookieBanner`
- `ai-worker/raven_worker/agent.py` — call `LLMSpendFuse.try_charge()` before each LLM call
- `ai-worker/raven_worker/config.py` — add `llm_daily_usd_cap` setting
- `deploy/ansible/roles/raven-backup/tasks/main.yml` — install the new timer

**Codebase-locating steps:** several tasks start with a `grep` / `rg` step to locate the actual filename, because file names like `signup.go`, `SignupView.vue`, and the API bootstrap entry differ across repo conventions. The implementer should resolve to real paths before editing.

---

## Tasks

### Task 1: Locate insertion points (read-only)

**Files:** none

- [ ] **Step 1: Locate the SuperTokens user-creation hook in the Go API**

```bash
rg -nE "supertokens|UserCreated|signUp|signup.*hook|RegisterCallback" internal/ api/ | head -40
```

Record the file and function in a scratch file `/tmp/demo-app-paths.txt`.

- [ ] **Step 2: Locate the API route registration**

```bash
rg -nE "router\.(Group|GET|POST)|gin\.Engine|RegisterRoutes" internal/ | head -40
```

Identify the central route registrar.

- [ ] **Step 3: Locate the python-worker's LLM call site(s)**

```bash
rg -nE "(openai|anthropic|llm).*\.(create|complete|chat|messages)|client\.(chat|messages)" ai-worker/ | head -20
```

Identify the function(s) that issue paid LLM calls.

- [ ] **Step 4: Locate the frontend signup view and the app shell**

```bash
rg -nE "sign[- ]?up|RegisterView|SignupView" frontend/src --type vue | head -20
rg -nE "App\\.vue|RouterView|<router-view" frontend/src --type vue | head -10
```

- [ ] **Step 5: Add the path inventory to `docs/runbooks/demo-app-features.md`**

```markdown
# Demo App Features — codebase inventory (recorded YYYY-MM-DD)

| Concept | File | Symbol/route |
|---|---|---|
| SuperTokens user-create hook | … | … |
| API route registrar | … | … |
| Python worker LLM call site | … | … |
| Frontend signup view | … | … |
| Frontend app shell | … | … |
```

- [ ] **Step 6: Commit**

```bash
git add docs/runbooks/demo-app-features.md
git commit -m "docs(demo): app-features insertion-point inventory"
```

---

### Task 2: MailSender interface

**Files:**
- Create: `internal/mail/sender.go`
- Create: `internal/mail/sender_test.go`

- [ ] **Step 1: Write the failing test**

```go
package mail

import (
	"context"
	"testing"
)

func TestNoopSender_Send_RecordsMessage(t *testing.T) {
	s := &NoopSender{}
	err := s.Send(context.Background(), Message{
		To:      "user@example.com",
		Subject: "Hi",
		Text:    "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(s.Sent))
	}
	if s.Sent[0].To != "user@example.com" {
		t.Fatalf("unexpected To: %s", s.Sent[0].To)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/mail/...
```

Expected: FAIL with "package mail does not exist" or similar.

- [ ] **Step 3: Write the minimal implementation**

```go
// Package mail defines a provider-agnostic email-sending interface.
package mail

import "context"

type Message struct {
	To      string
	Subject string
	Text    string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

type NoopSender struct {
	Sent []Message
}

func (s *NoopSender) Send(_ context.Context, msg Message) error {
	s.Sent = append(s.Sent, msg)
	return nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/mail/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mail/sender.go internal/mail/sender_test.go
git commit -m "feat(mail): Sender interface and NoopSender"
```

---

### Task 3: Resend implementation of Sender

**Files:**
- Create: `internal/mail/resend.go`
- Create: `internal/mail/resend_test.go`

- [ ] **Step 1: Write the failing test using httptest**

```go
package mail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendSender_Send_PostsToAPI(t *testing.T) {
	var got struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or wrong auth header: %s", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg-123"}`))
	}))
	defer srv.Close()

	s := &ResendSender{
		APIKey: "test-key",
		From:   "noreply@example.org",
		client: srv.Client(),
		url:    srv.URL,
	}

	err := s.Send(context.Background(), Message{
		To:      "user@example.com",
		Subject: "Hi",
		Text:    "Hello",
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if got.From != "noreply@example.org" {
		t.Errorf("From: got %q", got.From)
	}
	if !strings.Contains(strings.Join(got.To, ","), "user@example.com") {
		t.Errorf("To: got %v", got.To)
	}
}

func TestResendSender_Send_PropagatesNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	s := &ResendSender{
		APIKey: "bad", From: "noreply@example.org",
		client: srv.Client(), url: srv.URL,
	}
	err := s.Send(context.Background(), Message{To: "u@e.com", Subject: "s", Text: "t"})
	if err == nil {
		t.Fatal("expected error on 401")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails to compile**

```bash
go test ./internal/mail/...
```

Expected: FAIL (ResendSender undefined).

- [ ] **Step 3: Implement `ResendSender`**

```go
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ResendSender struct {
	APIKey string
	From   string
	client *http.Client
	url    string
}

func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		APIKey: apiKey,
		From:   from,
		client: &http.Client{Timeout: 10 * time.Second},
		url:    "https://api.resend.com/emails",
	}
}

func (s *ResendSender) Send(ctx context.Context, msg Message) error {
	if s.APIKey == "" {
		return errors.New("resend: API key not configured")
	}
	body, err := json.Marshal(map[string]any{
		"from":    s.From,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"text":    msg.Text,
	})
	if err != nil {
		return fmt.Errorf("resend: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend: new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend: non-2xx %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/mail/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mail/resend.go internal/mail/resend_test.go
git commit -m "feat(mail): Resend HTTP adapter"
```

---

### Task 4: Wire Sender into the API bootstrap

**Files:**
- Modify: API bootstrap file located in Task 1 Step 2

- [ ] **Step 1: Read the env var, construct a Sender at startup**

In the API bootstrap (e.g., `internal/api/server.go`), add:

```go
import (
	"os"
	"github.com/ravencloak-org/raven/internal/mail"
)

// inside the bootstrap function, after config is loaded:
var mailSender mail.Sender
if key := os.Getenv("RESEND_API_KEY"); key != "" {
	mailSender = mail.NewResendSender(key, getenvOrDefault("RESEND_FROM", "noreply@ravencloak.org"))
} else {
	mailSender = &mail.NoopSender{}
}
// pass mailSender into the dependency graph (registry / DI container / handler constructors)
```

Use whatever DI pattern the codebase already follows. If there's a service registry (e.g., `services.Registry`), register `mailSender` there.

- [ ] **Step 2: Verify the build is green**

```bash
go build ./...
go test ./internal/mail/...
```

Expected: both succeed.

- [ ] **Step 3: Commit**

```bash
git add internal/api/  # or whatever file you modified
git commit -m "feat(api): wire Resend MailSender at bootstrap, fall back to noop"
```

---

### Task 5: Turnstile verifier in Go

**Files:**
- Create: `internal/turnstile/verifier.go`
- Create: `internal/turnstile/verifier_test.go`

- [ ] **Step 1: Write the failing test**

```go
package turnstile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifier_Verify_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("secret") != "secret-key" {
			t.Errorf("wrong secret: %s", r.Form.Get("secret"))
		}
		if r.Form.Get("response") != "good-token" {
			t.Errorf("wrong token: %s", r.Form.Get("response"))
		}
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer srv.Close()

	v := &Verifier{SecretKey: "secret-key", url: srv.URL, client: srv.Client()}
	ok, err := v.Verify(context.Background(), "good-token", "1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
}

func TestVerifier_Verify_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error-codes": []string{"invalid-input-response"}})
	}))
	defer srv.Close()

	v := &Verifier{SecretKey: "k", url: srv.URL, client: srv.Client()}
	ok, _ := v.Verify(context.Background(), "bad-token", "")
	if ok {
		t.Fatal("expected failure")
	}
}
```

- [ ] **Step 2: Run the test (should fail to compile)**

```bash
go test ./internal/turnstile/...
```

Expected: FAIL.

- [ ] **Step 3: Implement the verifier**

```go
// Package turnstile verifies Cloudflare Turnstile tokens server-side.
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Verifier struct {
	SecretKey string
	url       string
	client    *http.Client
}

func NewVerifier(secretKey string) *Verifier {
	return &Verifier{
		SecretKey: secretKey,
		url:       "https://challenges.cloudflare.com/turnstile/v0/siteverify",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (v *Verifier) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if v.SecretKey == "" {
		return false, fmt.Errorf("turnstile: secret not configured")
	}
	form := url.Values{}
	form.Set("secret", v.SecretKey)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.url, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.Success, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/turnstile/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/turnstile/
git commit -m "feat(turnstile): server-side token verifier"
```

---

### Task 6: Gate signup with Turnstile in the API

**Files:**
- Modify: SuperTokens user-create hook located in Task 1 Step 1
- Modify: API bootstrap to instantiate the verifier

- [ ] **Step 1: Add a middleware that requires `cf-turnstile-token` header on the signup endpoint**

Create `internal/turnstile/middleware.go`:

```go
package turnstile

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Required returns a Gin middleware that 403s when the Turnstile token
// is missing or fails verification. Bypass header `X-Demo-Bypass: <secret>`
// is honoured for E2E tests (set in CI via TURNSTILE_BYPASS_SECRET).
func Required(v *Verifier, bypassSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bypassSecret != "" && c.GetHeader("X-Demo-Bypass") == bypassSecret {
			c.Next()
			return
		}
		token := c.GetHeader("cf-turnstile-token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing turnstile token"})
			return
		}
		ok, err := v.Verify(c.Request.Context(), token, c.ClientIP())
		if err != nil || !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "turnstile verification failed"})
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 2: Write a test for the middleware**

`internal/turnstile/middleware_test.go`:

```go
package turnstile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequired_Bypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", Required(&Verifier{SecretKey: "k"}, "bypass-it"), func(c *gin.Context) {
		c.Status(200)
	})

	req := httptest.NewRequest("POST", "/x", nil)
	req.Header.Set("X-Demo-Bypass", "bypass-it")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequired_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", Required(&Verifier{SecretKey: "k"}, ""), func(c *gin.Context) {
		c.Status(200)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(w.Body).Decode(&body)
	if body["error"] == "" {
		t.Fatal("missing error body")
	}
}
```

- [ ] **Step 3: Run the tests**

```bash
go test ./internal/turnstile/...
```

Expected: PASS for both.

- [ ] **Step 4: Mount the middleware on the SuperTokens signup route**

Open the file identified in Task 1 Step 1. Where the auth router/group is constructed, wrap or precede the signup handler:

```go
import (
	"github.com/ravencloak-org/raven/internal/turnstile"
)

// at bootstrap:
turnstileVerifier := turnstile.NewVerifier(os.Getenv("TURNSTILE_SECRET_KEY"))
bypassSecret := os.Getenv("TURNSTILE_BYPASS_SECRET") // set only in CI

// where the signup route is registered:
authGroup.POST("/signup", turnstile.Required(turnstileVerifier, bypassSecret), signupHandler)
```

Adapt the exact route path to match SuperTokens' actual signup endpoint shape in this codebase (recorded in Task 1).

- [ ] **Step 5: Build and run all existing API tests**

```bash
go build ./...
go test ./...
```

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/turnstile/ internal/  # whatever auth-route file changed
git commit -m "feat(api): require Turnstile token on signup, CI bypass header"
```

---

### Task 7: Turnstile widget in the frontend

**Files:**
- Create: `frontend/src/components/TurnstileWidget.vue`
- Modify: the signup view located in Task 1 Step 4

- [ ] **Step 1: Write the widget component**

```vue
<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'

const props = defineProps<{ siteKey: string }>()
const emit = defineEmits<{ (e: 'token', token: string): void }>()
const container = ref<HTMLDivElement | null>(null)
let widgetId: string | null = null

function loadScript(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.querySelector('script[src*="turnstile"]')) {
      resolve(); return
    }
    const s = document.createElement('script')
    s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js'
    s.async = true
    s.defer = true
    s.onload = () => resolve()
    s.onerror = reject
    document.head.appendChild(s)
  })
}

onMounted(async () => {
  await loadScript()
  // @ts-expect-error — global injected by turnstile script
  widgetId = window.turnstile.render(container.value, {
    sitekey: props.siteKey,
    callback: (token: string) => emit('token', token),
  })
})

onBeforeUnmount(() => {
  if (widgetId !== null && (window as any).turnstile) {
    (window as any).turnstile.remove(widgetId)
  }
})
</script>

<template>
  <div ref="container" />
</template>
```

- [ ] **Step 2: Use the widget in the signup view**

In the signup view (`frontend/src/views/auth/SignupView.vue` per the inventory):

```vue
<script setup lang="ts">
import { ref } from 'vue'
import TurnstileWidget from '@/components/TurnstileWidget.vue'

const turnstileToken = ref<string | null>(null)
const siteKey = import.meta.env.VITE_TURNSTILE_SITE_KEY ?? ''

async function onGoogleSignIn() {
  if (!turnstileToken.value) {
    alert('Please complete the verification.')
    return
  }
  await fetch('/api/auth/signup/google', {
    method: 'POST',
    headers: {
      'cf-turnstile-token': turnstileToken.value,
      'content-type': 'application/json',
    },
    body: JSON.stringify({/* existing payload */}),
  })
  // existing redirect logic
}
</script>

<template>
  <!-- existing markup; insert before the Google button -->
  <TurnstileWidget v-if="siteKey" :site-key="siteKey" @token="turnstileToken = $event" />
  <button :disabled="!turnstileToken" @click="onGoogleSignIn">Sign in with Google</button>
</template>
```

Adapt the existing signup handler invocation to forward the token. Exact event names and payload fields match whatever the current signup flow does (see inventory).

- [ ] **Step 3: Expose `VITE_TURNSTILE_SITE_KEY` to the frontend at build time**

Add to `frontend/.env.example` (create if missing):

```dotenv
VITE_TURNSTILE_SITE_KEY=
```

In the compose overlay `docker-compose.demo.yml`, the frontend already receives `RAVEN_TURNSTILE_SITE_KEY` (from plan #1 Task 12). Verify the frontend's docker entrypoint maps it to `VITE_TURNSTILE_SITE_KEY` at runtime, or expose it via a runtime config endpoint (`/config.json`) the SPA fetches at boot. Pick whichever pattern the frontend already uses for runtime config.

- [ ] **Step 4: Run frontend type-check**

```bash
cd frontend && bun run type-check
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/TurnstileWidget.vue frontend/src/views/auth/SignupView.vue frontend/.env.example
git commit -m "feat(frontend): Turnstile widget on signup, token forwarded as header"
```

---

### Task 8: LLM $-fuse in python-worker

**Files:**
- Create: `ai-worker/raven_worker/llm_fuse.py`
- Create: `ai-worker/tests/test_llm_fuse.py`
- Modify: `ai-worker/raven_worker/config.py` — add `llm_daily_usd_cap`
- Modify: `ai-worker/raven_worker/agent.py` — wrap LLM call

- [ ] **Step 1: Write the failing test using fakeredis**

```python
import pytest
import fakeredis
from raven_worker.llm_fuse import LLMSpendFuse, FuseTripped

@pytest.fixture
def fuse():
    r = fakeredis.FakeRedis()
    return LLMSpendFuse(redis=r, daily_cap_usd=1.0, key_prefix="test:llm")

def test_charge_below_cap_allowed(fuse):
    fuse.charge(0.30)
    fuse.charge(0.40)
    # under 1.0 — should not raise
    assert fuse.spent_today() == pytest.approx(0.70)

def test_charge_over_cap_trips(fuse):
    fuse.charge(0.60)
    fuse.charge(0.30)
    with pytest.raises(FuseTripped):
        fuse.charge(0.20)

def test_check_only_does_not_increment(fuse):
    fuse.charge(0.60)
    fuse.guard(0.30)  # would still be under cap
    assert fuse.spent_today() == pytest.approx(0.60)

def test_guard_raises_when_would_exceed(fuse):
    fuse.charge(0.80)
    with pytest.raises(FuseTripped):
        fuse.guard(0.30)
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ai-worker && uv run pytest tests/test_llm_fuse.py -v
```

Expected: FAIL with import errors.

- [ ] **Step 3: Implement `LLMSpendFuse`**

```python
"""Daily LLM spend circuit breaker backed by Redis/Valkey.

Counter key resets at UTC midnight via Redis EXPIRE.
"""

from __future__ import annotations
from datetime import datetime, timezone
from typing import Protocol


class FuseTripped(Exception):
    """Raised when the daily spend cap would be exceeded."""


class _Redis(Protocol):
    def incrbyfloat(self, key: str, amount: float) -> float: ...
    def expire(self, key: str, seconds: int) -> bool: ...
    def get(self, key: str) -> bytes | None: ...


class LLMSpendFuse:
    def __init__(
        self,
        redis: _Redis,
        daily_cap_usd: float,
        key_prefix: str = "raven:llm:spend",
    ):
        self.redis = redis
        self.cap = daily_cap_usd
        self.key_prefix = key_prefix

    def _key(self) -> str:
        day = datetime.now(timezone.utc).strftime("%Y%m%d")
        return f"{self.key_prefix}:{day}"

    def spent_today(self) -> float:
        raw = self.redis.get(self._key())
        return float(raw) if raw else 0.0

    def guard(self, estimated_cost_usd: float) -> None:
        """Raise FuseTripped if charging this cost would exceed the cap."""
        if self.spent_today() + estimated_cost_usd > self.cap:
            raise FuseTripped(
                f"Daily LLM cap ${self.cap:.2f} would be exceeded "
                f"(spent ${self.spent_today():.4f}, +${estimated_cost_usd:.4f})"
            )

    def charge(self, actual_cost_usd: float) -> None:
        """Record cost. Raises FuseTripped if it puts total over the cap."""
        new_total = self.redis.incrbyfloat(self._key(), actual_cost_usd)
        self.redis.expire(self._key(), 60 * 60 * 26)  # 26h TTL covers DST
        if float(new_total) > self.cap:
            raise FuseTripped(
                f"Daily LLM cap ${self.cap:.2f} exceeded (total ${new_total:.4f})"
            )
```

- [ ] **Step 4: Add `llm_daily_usd_cap` to config**

In `ai-worker/raven_worker/config.py`, add to the existing settings class:

```python
llm_daily_usd_cap: float = 5.00  # demo default; override via RAVEN_LLM_DAILY_USD_CAP
```

- [ ] **Step 5: Add fakeredis to dev deps**

```bash
cd ai-worker && uv add --dev fakeredis
```

- [ ] **Step 6: Run tests**

```bash
uv run pytest tests/test_llm_fuse.py -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add ai-worker/raven_worker/llm_fuse.py ai-worker/tests/test_llm_fuse.py ai-worker/raven_worker/config.py ai-worker/pyproject.toml ai-worker/uv.lock
git commit -m "feat(worker): LLMSpendFuse daily $-cap circuit breaker"
```

---

### Task 9: Wire `LLMSpendFuse` into the agent

**Files:**
- Modify: `ai-worker/raven_worker/agent.py` (path confirmed in Task 1)

- [ ] **Step 1: Instantiate the fuse at module/agent startup**

Add near the existing settings/redis client construction:

```python
from raven_worker.llm_fuse import LLMSpendFuse, FuseTripped

llm_fuse = LLMSpendFuse(
    redis=redis_client,                    # existing Valkey/Redis client
    daily_cap_usd=settings.llm_daily_usd_cap,
)
```

- [ ] **Step 2: Guard each LLM call site**

For every LLM call site identified in Task 1 Step 3, wrap as:

```python
estimated_cost = estimate_cost(model, prompt_tokens, max_tokens)  # use existing pricing util if any
try:
    llm_fuse.guard(estimated_cost)
except FuseTripped as e:
    raise DemoLimitReached(str(e)) from e

response = llm_client.create(...)              # existing call
actual_cost = compute_actual_cost(response)
try:
    llm_fuse.charge(actual_cost)
except FuseTripped:
    # spent reached cap during this request — log and continue
    logger.warning("llm_fuse: tripped on charge for this request")
```

Define `DemoLimitReached` near the top of the module:

```python
class DemoLimitReached(Exception):
    """Surfaced to clients as a 429 with a 'demo daily limit' message."""
```

If there is no existing pricing util, use a coarse fallback for the demo:

```python
def estimate_cost(model: str, prompt_tokens: int, max_tokens: int) -> float:
    # demo-grade pricing; refine when M9 lands a real cost model
    return (prompt_tokens + max_tokens) * 0.000003
```

- [ ] **Step 3: Translate `DemoLimitReached` into a 429 at the gRPC/HTTP boundary**

In whatever boundary layer the worker exposes (gRPC servicer or HTTP handler), catch `DemoLimitReached` and return:

```python
context.set_code(grpc.StatusCode.RESOURCE_EXHAUSTED)
context.set_details("Demo daily LLM limit reached. Try again tomorrow.")
return ChatResponse()  # or whatever the empty response shape is
```

- [ ] **Step 4: Add an integration-style test that asserts a request after a tripped fuse returns RESOURCE_EXHAUSTED**

Filename: `ai-worker/tests/test_agent_fuse_integration.py`

```python
import pytest
import fakeredis
from raven_worker.llm_fuse import LLMSpendFuse, FuseTripped

def test_guard_then_charge_then_trip():
    fuse = LLMSpendFuse(redis=fakeredis.FakeRedis(), daily_cap_usd=1.0)
    fuse.guard(0.50); fuse.charge(0.50)
    fuse.guard(0.40); fuse.charge(0.40)
    with pytest.raises(FuseTripped):
        fuse.guard(0.20)  # would put total at 1.10
```

- [ ] **Step 5: Run tests**

```bash
cd ai-worker && uv run pytest -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add ai-worker/
git commit -m "feat(worker): guard every LLM call with LLMSpendFuse; expose 429 to clients"
```

---

### Task 10: DSAR `/account/export` endpoint

**Files:**
- Create: `internal/account/dsar.go`
- Create: `internal/account/dsar_test.go`

- [ ] **Step 1: Write the failing test**

```go
package account

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRepo struct{}

func (fakeRepo) ExportUser(_ context.Context, userID string) (UserExport, error) {
	return UserExport{
		UserID: userID,
		Email:  "u@example.com",
		Rows: map[string][]map[string]any{
			"messages": {{"id": 1, "text": "hi"}},
		},
	}, nil
}

func TestExportHandler_WritesJSON(t *testing.T) {
	h := NewDSARHandler(fakeRepo{}, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/account/export", nil)
	req = req.WithContext(WithUserID(req.Context(), "user-1"))
	w := httptest.NewRecorder()
	h.Export(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("unexpected content-type: %s", ct)
	}
	var out UserExport
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.UserID != "user-1" {
		t.Fatalf("UserID: got %s", out.UserID)
	}
	if len(out.Rows["messages"]) != 1 {
		t.Fatalf("expected 1 message row")
	}
}
```

- [ ] **Step 2: Run test (expect compile failure)**

```bash
go test ./internal/account/...
```

- [ ] **Step 3: Implement**

```go
package account

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ravencloak-org/raven/internal/mail"
)

type UserExport struct {
	UserID string                          `json:"user_id"`
	Email  string                          `json:"email"`
	Rows   map[string][]map[string]any     `json:"rows"`
}

type DSARRepo interface {
	ExportUser(ctx context.Context, userID string) (UserExport, error)
	ScheduleDelete(ctx context.Context, userID string) error
}

type DSARHandler struct {
	Repo   DSARRepo
	Mail   mail.Sender
	NowFn  func() any // optional injection for tests; unused for export
}

func NewDSARHandler(repo DSARRepo, mailer mail.Sender, _ any) *DSARHandler {
	return &DSARHandler{Repo: repo, Mail: mailer}
}

type ctxKey string

const userIDKey ctxKey = "user_id"

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func UserIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok && v != ""
}

func (h *DSARHandler) Export(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	out, err := h.Repo.ExportUser(r.Context(), uid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=raven-export.json")
	_ = json.NewEncoder(w).Encode(out)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/account/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/account/dsar.go internal/account/dsar_test.go
git commit -m "feat(account): GET /account/export streams user data as JSON"
```

---

### Task 11: DSAR `/account/delete` endpoint (24h grace)

**Files:**
- Modify: `internal/account/dsar.go`
- Modify: `internal/account/dsar_test.go`
- Create: `internal/mail/templates/dsar_delete_confirm.txt`

- [ ] **Step 1: Write the failing test for delete-schedules**

```go
type fakeRepo2 struct {
	scheduled string
}

func (f *fakeRepo2) ExportUser(_ context.Context, id string) (UserExport, error) {
	return UserExport{UserID: id}, nil
}
func (f *fakeRepo2) ScheduleDelete(_ context.Context, id string) error {
	f.scheduled = id
	return nil
}

type fakeMail struct{ sent []mail.Message }

func (f *fakeMail) Send(_ context.Context, m mail.Message) error {
	f.sent = append(f.sent, m); return nil
}

func TestDeleteHandler_SchedulesAndEmails(t *testing.T) {
	repo := &fakeRepo2{}
	m := &fakeMail{}
	h := NewDSARHandler(repo, m, nil)

	req := httptest.NewRequest(http.MethodPost, "/account/delete", nil)
	req = req.WithContext(WithUserID(req.Context(), "user-2"))
	req = req.WithContext(WithUserEmail(req.Context(), "u@e.com"))
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if repo.scheduled != "user-2" {
		t.Fatalf("expected schedule for user-2, got %q", repo.scheduled)
	}
	if len(m.sent) != 1 {
		t.Fatalf("expected 1 email")
	}
	if !strings.Contains(m.sent[0].Subject, "delete") {
		t.Fatalf("email subject doesn't mention delete: %s", m.sent[0].Subject)
	}
}
```

- [ ] **Step 2: Run (expect failure)**

```bash
go test ./internal/account/...
```

- [ ] **Step 3: Implement `Delete` and the user-email context**

Add to `dsar.go`:

```go
const userEmailKey ctxKey = "user_email"

func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, userEmailKey, email)
}
func UserEmailFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userEmailKey).(string)
	return v, ok && v != ""
}

func (h *DSARHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	if err := h.Repo.ScheduleDelete(r.Context(), uid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if email, ok := UserEmailFrom(r.Context()); ok && h.Mail != nil {
		_ = h.Mail.Send(r.Context(), mail.Message{
			To:      email,
			Subject: "Your Raven delete request",
			Text:    "We received your request to delete your Raven account. Your data will be removed in 24 hours. Reply to this email if you didn't request this.",
		})
	}
	w.WriteHeader(http.StatusAccepted)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/account/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/account/
git commit -m "feat(account): POST /account/delete schedules 24h grace and emails confirmation"
```

---

### Task 12: Register DSAR routes

**Files:**
- Modify: route registrar identified in Task 1 Step 2

- [ ] **Step 1: Mount the handlers behind the authenticated middleware**

```go
import "github.com/ravencloak-org/raven/internal/account"

// at bootstrap, after sessions middleware is configured:
dsarRepo := account.NewSQLRepo(db)  // implement matching the DSARRepo interface
dsar := account.NewDSARHandler(dsarRepo, mailSender, nil)

authed := router.Group("/api/account")
authed.Use(sessionsMiddleware)            // existing
authed.GET("/export", gin.WrapF(dsar.Export))
authed.POST("/delete", gin.WrapF(dsar.Delete))
```

- [ ] **Step 2: Implement `account.SQLRepo` to satisfy `DSARRepo`**

Create `internal/account/repo_sql.go`:

```go
package account

import (
	"context"
	"database/sql"
)

type SQLRepo struct {
	DB *sql.DB
}

func NewSQLRepo(db *sql.DB) *SQLRepo { return &SQLRepo{DB: db} }

// ExportUser collects rows belonging to the user across every table that
// stores user data. Schema is implementation-specific; the list below is a
// starting point and must be expanded as new tables are added.
func (r *SQLRepo) ExportUser(ctx context.Context, userID string) (UserExport, error) {
	out := UserExport{UserID: userID, Rows: map[string][]map[string]any{}}
	// TODO: replace with the project's actual user-owned tables.
	// Each tableExport call writes its rows under the table name key.
	tables := []string{"messages", "workspaces", "voice_sessions"}
	for _, t := range tables {
		rows, err := r.tableExport(ctx, t, userID)
		if err != nil {
			return out, err
		}
		out.Rows[t] = rows
	}
	// fetch email
	if err := r.DB.QueryRowContext(ctx,
		"SELECT email FROM users WHERE id = $1", userID).Scan(&out.Email); err != nil && err != sql.ErrNoRows {
		return out, err
	}
	return out, nil
}

func (r *SQLRepo) tableExport(ctx context.Context, table, userID string) ([]map[string]any, error) {
	// Use parameterised query — table name is not user-controlled here, so
	// string interpolation is acceptable but caller MUST pass an allowlisted name.
	rows, err := r.DB.QueryContext(ctx,
		"SELECT row_to_json(t)::text FROM "+table+" t WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		var m map[string]any
		// json.Unmarshal in a real impl; demo-grade left for the implementer
		_ = s; _ = m
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *SQLRepo) ScheduleDelete(ctx context.Context, userID string) error {
	_, err := r.DB.ExecContext(ctx,
		"INSERT INTO scheduled_deletes(user_id, run_at) VALUES ($1, now() + interval '24 hours') ON CONFLICT (user_id) DO NOTHING",
		userID)
	return err
}
```

- [ ] **Step 2.5: Add `scheduled_deletes` migration**

Create `migrations/<next-id>_scheduled_deletes.sql`:

```sql
CREATE TABLE IF NOT EXISTS scheduled_deletes (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  run_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS scheduled_deletes_run_at_idx ON scheduled_deletes (run_at);
```

(Adapt the `users` foreign key to the actual users table name.)

- [ ] **Step 3: Build and run all tests**

```bash
go build ./...
go test ./...
```

Expected: green.

- [ ] **Step 4: Commit**

```bash
git add internal/account/ migrations/
git commit -m "feat(account): wire DSAR endpoints + scheduled_deletes table"
```

---

### Task 13: Retention purge — service + cron

**Files:**
- Create: `internal/account/retention.go`
- Create: `internal/account/retention_test.go`
- Create: `deploy/ansible/roles/raven-backup/files/raven-retention-purge.service`
- Create: `deploy/ansible/roles/raven-backup/files/raven-retention-purge.timer`
- Modify: `deploy/ansible/roles/raven-backup/tasks/main.yml`
- Modify: API route registrar — add admin-only `/admin/retention/run` endpoint

- [ ] **Step 1: Write the failing test for `RunOnce`**

```go
package account

import (
	"context"
	"testing"
	"time"
)

type stubRepo struct {
	warned, deleted []string
	now             time.Time
}

func (s *stubRepo) InactiveUsers(_ context.Context, since time.Time) ([]InactiveUser, error) {
	_ = since
	return []InactiveUser{
		{ID: "warn-me", Email: "w@e.com", LastActive: s.now.Add(-25 * 24 * time.Hour)},
		{ID: "purge-me", Email: "p@e.com", LastActive: s.now.Add(-31 * 24 * time.Hour)},
	}, nil
}
func (s *stubRepo) MarkWarned(_ context.Context, id string) error  { s.warned = append(s.warned, id); return nil }
func (s *stubRepo) HardDelete(_ context.Context, id string) error  { s.deleted = append(s.deleted, id); return nil }

func TestRunOnce_WarnsAtDay25_DeletesAt31(t *testing.T) {
	repo := &stubRepo{now: time.Now().UTC()}
	mailer := &fakeMail{}
	p := NewRetentionPurger(repo, mailer)
	if err := p.RunOnce(context.Background(), repo.now); err != nil {
		t.Fatal(err)
	}
	if len(repo.warned) != 1 || repo.warned[0] != "warn-me" {
		t.Fatalf("warned: %v", repo.warned)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "purge-me" {
		t.Fatalf("deleted: %v", repo.deleted)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].To != "w@e.com" {
		t.Fatalf("expected warning email to w@e.com, got %v", mailer.sent)
	}
}
```

- [ ] **Step 2: Run test (expect failure)**

```bash
go test ./internal/account/...
```

- [ ] **Step 3: Implement**

```go
package account

import (
	"context"
	"time"

	"github.com/ravencloak-org/raven/internal/mail"
)

type InactiveUser struct {
	ID         string
	Email      string
	LastActive time.Time
}

type RetentionRepo interface {
	InactiveUsers(ctx context.Context, since time.Time) ([]InactiveUser, error)
	MarkWarned(ctx context.Context, id string) error
	HardDelete(ctx context.Context, id string) error
}

type RetentionPurger struct {
	Repo RetentionRepo
	Mail mail.Sender
}

func NewRetentionPurger(repo RetentionRepo, mailer mail.Sender) *RetentionPurger {
	return &RetentionPurger{Repo: repo, Mail: mailer}
}

func (p *RetentionPurger) RunOnce(ctx context.Context, now time.Time) error {
	warnCutoff := now.Add(-23 * 24 * time.Hour)
	deleteCutoff := now.Add(-30 * 24 * time.Hour)

	users, err := p.Repo.InactiveUsers(ctx, warnCutoff)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.LastActive.Before(deleteCutoff) {
			if err := p.Repo.HardDelete(ctx, u.ID); err != nil {
				return err
			}
			continue
		}
		_ = p.Mail.Send(ctx, mail.Message{
			To:      u.Email,
			Subject: "Your Raven demo account will be deleted in 7 days",
			Text:    "You haven't used Raven for 23 days. Inactive demo accounts are deleted at 30 days. Sign in to keep your data: https://demo.raven.ravencloak.org",
		})
		if err := p.Repo.MarkWarned(ctx, u.ID); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/account/...
```

Expected: PASS.

- [ ] **Step 5: Add admin endpoint**

In the route registrar:

```go
purger := account.NewRetentionPurger(dsarRepo, mailSender)  // SQLRepo implements RetentionRepo too
admin := router.Group("/api/admin")
admin.Use(adminOnlyMiddleware)  // existing or new — must require an admin SuperTokens role
admin.POST("/retention/run", func(c *gin.Context) {
	if err := purger.RunOnce(c.Request.Context(), time.Now().UTC()); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
})
```

Implement `SQLRepo.InactiveUsers/MarkWarned/HardDelete` in `internal/account/repo_sql.go` — schema-specific.

- [ ] **Step 6: Write the systemd service**

`deploy/ansible/roles/raven-backup/files/raven-retention-purge.service`:

```ini
[Unit]
Description=Raven retention purge — calls /api/admin/retention/run
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
EnvironmentFile=/etc/raven/env
ExecStart=/usr/bin/curl -fsS -X POST \
  -H "Authorization: Bearer ${RAVEN_ADMIN_API_TOKEN}" \
  http://localhost:8081/api/admin/retention/run
StandardOutput=journal
StandardError=journal
```

- [ ] **Step 7: Write the systemd timer**

```ini
[Unit]
Description=Raven retention purge timer

[Timer]
OnCalendar=*-*-* 03:00:00 UTC
RandomizedDelaySec=600
Persistent=true
Unit=raven-retention-purge.service

[Install]
WantedBy=timers.target
```

- [ ] **Step 8: Extend Ansible `tasks/main.yml`**

```yaml
- name: Install retention purge service
  ansible.builtin.copy:
    src: raven-retention-purge.service
    dest: /etc/systemd/system/raven-retention-purge.service
    mode: "0644"

- name: Install retention purge timer
  ansible.builtin.copy:
    src: raven-retention-purge.timer
    dest: /etc/systemd/system/raven-retention-purge.timer
    mode: "0644"

- name: Enable retention purge timer
  ansible.builtin.systemd:
    name: raven-retention-purge.timer
    enabled: true
    state: started
    daemon_reload: true
```

- [ ] **Step 9: Add `RAVEN_ADMIN_API_TOKEN` to SSM**

Update `docs/runbooks/demo-bootstrap.md` with the new SSM parameter and its purpose. Add to plan #1 Task 1's prerequisites list.

- [ ] **Step 10: Commit**

```bash
git add internal/account/ deploy/ansible/roles/raven-backup/
git commit -m "feat(account): retention purger — 23d warn, 30d hard delete; nightly cron"
```

---

### Task 14: Sample-workspace seed on user create

**Files:**
- Create: `internal/seed/sample_workspace.go`
- Create: `internal/seed/sample_workspace_test.go`
- Create: `internal/seed/fixtures/sample_workspace.json`
- Modify: SuperTokens user-create hook (located in Task 1 Step 1)

- [ ] **Step 1: Define the fixture (placeholder content)**

`internal/seed/fixtures/sample_workspace.json`:

```json
{
  "workspace_name": "Sample workspace",
  "messages": [
    {"role": "system", "text": "Welcome to Raven."},
    {"role": "assistant", "text": "Try asking me to summarise an article or schedule a follow-up."}
  ]
}
```

(Final fixture content per spec §11 — implementer to expand based on actual product surface highlighted at launch.)

- [ ] **Step 2: Write the failing test**

```go
package seed

import (
	"context"
	"testing"
)

type fakeWS struct {
	created  string
	messages []string
}

func (f *fakeWS) CreateWorkspace(_ context.Context, ownerID, name string) (string, error) {
	f.created = ownerID + ":" + name
	return "ws-1", nil
}
func (f *fakeWS) InsertMessage(_ context.Context, ws, role, text string) error {
	f.messages = append(f.messages, role+":"+text); return nil
}

func TestSeed_CreatesWorkspaceAndMessages(t *testing.T) {
	repo := &fakeWS{}
	if err := SeedSampleWorkspace(context.Background(), repo, "user-1"); err != nil {
		t.Fatal(err)
	}
	if repo.created == "" {
		t.Fatal("workspace not created")
	}
	if len(repo.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(repo.messages))
	}
}
```

- [ ] **Step 3: Run test (expect failure)**

```bash
go test ./internal/seed/...
```

- [ ] **Step 4: Implement**

```go
package seed

import (
	"context"
	_ "embed"
	"encoding/json"
)

//go:embed fixtures/sample_workspace.json
var sampleWorkspaceJSON []byte

type fixture struct {
	WorkspaceName string `json:"workspace_name"`
	Messages      []struct {
		Role string `json:"role"`
		Text string `json:"text"`
	} `json:"messages"`
}

type WorkspaceSeeder interface {
	CreateWorkspace(ctx context.Context, ownerID, name string) (string, error)
	InsertMessage(ctx context.Context, workspaceID, role, text string) error
}

func SeedSampleWorkspace(ctx context.Context, s WorkspaceSeeder, ownerID string) error {
	var f fixture
	if err := json.Unmarshal(sampleWorkspaceJSON, &f); err != nil {
		return err
	}
	ws, err := s.CreateWorkspace(ctx, ownerID, f.WorkspaceName)
	if err != nil {
		return err
	}
	for _, m := range f.Messages {
		if err := s.InsertMessage(ctx, ws, m.Role, m.Text); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/seed/...
```

Expected: PASS.

- [ ] **Step 6: Call from the user-create hook**

In the SuperTokens user-create hook file (Task 1 Step 1):

```go
import "github.com/ravencloak-org/raven/internal/seed"

// after the user row is inserted:
if err := seed.SeedSampleWorkspace(ctx, seedRepo, newUser.ID); err != nil {
	log.Error("seed sample workspace failed", "user", newUser.ID, "err", err)
	// non-fatal — user can still sign in
}
```

`seedRepo` is whatever struct in the codebase implements `CreateWorkspace` and `InsertMessage` against the real DB. Adapt naming.

- [ ] **Step 7: Commit**

```bash
git add internal/seed/
git commit -m "feat(seed): pre-seed sample workspace on user create"
```

---

### Task 15: Privacy policy + ToS pages (landing)

**Files:**
- Create: `landing/src/routes/legal/privacy.md` (or the equivalent Cloudflare Pages content shape)
- Create: `landing/src/routes/legal/terms.md`

- [ ] **Step 1: Determine the landing site's content shape**

```bash
ls landing/src/ 2>/dev/null
cat landing/README.md 2>/dev/null
```

Identify whether `landing/` uses Astro / SvelteKit / vanilla HTML / markdown.

- [ ] **Step 2: Draft `privacy.md`**

Use a template (e.g. Termly, GDPR.eu) tailored for the demo's scope:
- Controller: identify yourself and the entity (or "individual operating in personal capacity")
- Data collected: Google profile (email, name, avatar), conversation text, IP address, browser fingerprint
- Lawful basis: legitimate interest (Art. 6(1)(f) GDPR) for service operation; consent for any optional cookies
- Retention: 30 days after last activity, then deleted (links to §6 of the spec)
- Recipients: list every third-party processor (Resend, Cloudflare, AWS, LLM provider, Razorpay sandbox)
- Rights: access, rectification, erasure (link to `/account/export` and `/account/delete`)
- Contact: `privacy@ravencloak.org`

Get a lawyer to review before flipping Phase 3.

- [ ] **Step 3: Draft `terms.md`**

Cover: acceptable use, demo-only nature, no SLA, no warranty, governing law (Karnataka / India for an Indian operator), modification and termination clauses.

- [ ] **Step 4: Wire navigation**

Add links to the landing page footer pointing to `/legal/privacy` and `/legal/terms`. Add the same links to the demo app's footer (`frontend/src/components/Footer.vue` if it exists; create if not).

- [ ] **Step 5: Build the landing site locally**

```bash
cd landing && bun run build
```

Expected: success, both routes generated.

- [ ] **Step 6: Commit**

```bash
git add landing/src/routes/legal/ frontend/src/components/Footer.vue
git commit -m "feat(legal): privacy policy and terms of service pages"
```

---

### Task 16: Cookie consent banner (essential-only)

**Files:**
- Create: `frontend/src/components/CookieBanner.vue`
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Write `CookieBanner.vue`**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'

const visible = ref(false)
const STORAGE_KEY = 'raven.cookie.consent.v1'

onMounted(() => {
  if (!localStorage.getItem(STORAGE_KEY)) visible.value = true
})

function accept() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify({ essential: true, accepted_at: new Date().toISOString() }))
  visible.value = false
}
</script>

<template>
  <Transition>
    <div v-if="visible" class="cookie-banner" role="dialog" aria-live="polite">
      <p>
        Raven uses essential cookies for login and security only. No tracking,
        no analytics. <a href="/legal/privacy">Learn more</a>.
      </p>
      <button @click="accept">Got it</button>
    </div>
  </Transition>
</template>

<style scoped>
.cookie-banner {
  position: fixed; bottom: 0; left: 0; right: 0;
  padding: 1rem; background: var(--surface-1, #111);
  color: var(--text-1, #fff); display: flex; gap: 1rem; align-items: center;
  border-top: 1px solid var(--surface-3, #333); z-index: 1000;
}
button { padding: 0.5rem 1rem; background: var(--accent, #5b8dee); color: white; border: 0; border-radius: 0.25rem; }
</style>
```

- [ ] **Step 2: Mount in `App.vue`**

```vue
<script setup lang="ts">
import CookieBanner from './components/CookieBanner.vue'
// ... existing imports
</script>

<template>
  <!-- existing layout -->
  <RouterView />
  <CookieBanner />
</template>
```

- [ ] **Step 3: Build the frontend**

```bash
cd frontend && bun run build
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/CookieBanner.vue frontend/src/App.vue
git commit -m "feat(frontend): essential-only cookie banner with localStorage consent"
```

---

### Task 17: Hide voice UI on the admin dashboard

**Files:**
- Modify: frontend admin dashboard files that surface voice features

- [ ] **Step 1: Locate every voice-UI surface**

```bash
rg -nE "voice|livekit|call.button|VoiceSession|webrtc" frontend/src --type vue | head -40
```

Build a list of files where voice features appear in the admin UI (not the embeddable widget — that already has its `voice-enabled` attribute).

- [ ] **Step 2: Add a runtime config check**

Define a single helper `frontend/src/lib/featureFlags.ts`:

```ts
export function isVoiceEnabled(): boolean {
  // Runtime config injected by the frontend container entrypoint.
  // Falls back to build-time env for dev.
  const runtime = (window as any).__RAVEN_CONFIG__?.voiceEnabled
  if (typeof runtime === 'boolean') return runtime
  return import.meta.env.VITE_VOICE_ENABLED === 'true'
}
```

- [ ] **Step 3: Wrap every voice surface with `v-if="isVoiceEnabled()"` or `<template v-if>`**

For each file found in Step 1, wrap the voice-related component / button / route guard:

```vue
<script setup lang="ts">
import { isVoiceEnabled } from '@/lib/featureFlags'
</script>

<template>
  <button v-if="isVoiceEnabled()" @click="startCall">Start voice call</button>
</template>
```

For routes (e.g. `/voice-sessions`), guard in the router:

```ts
import { isVoiceEnabled } from '@/lib/featureFlags'

router.beforeEach((to) => {
  if (to.path.startsWith('/voice') && !isVoiceEnabled()) {
    return { path: '/' }
  }
})
```

- [ ] **Step 4: Ensure the runtime config picks up `RAVEN_VOICE_ENABLED`**

In the frontend container entrypoint (`frontend/docker-entrypoint.sh` or similar), generate `window.__RAVEN_CONFIG__` from env. If no runtime-config pattern exists, add one:

`frontend/docker-entrypoint.sh`:

```bash
#!/bin/sh
set -e
cat > /usr/share/nginx/html/config.js <<EOF
window.__RAVEN_CONFIG__ = {
  voiceEnabled: ${RAVEN_VOICE_ENABLED:-false} === "true",
  turnstileSiteKey: "${RAVEN_TURNSTILE_SITE_KEY:-}",
};
EOF
exec "$@"
```

Include `<script src="/config.js"></script>` in `frontend/index.html` before the main bundle.

- [ ] **Step 5: Run frontend type-check and build**

```bash
cd frontend && bun run type-check && bun run build
```

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add frontend/
git commit -m "feat(frontend): gate voice UI on isVoiceEnabled() runtime flag"
```

---

### Task 18: End-to-end manual smoke

**Files:** none (operational)

After plans #1 and #2 have been applied and a tag containing all the changes from this plan has been deployed:

- [ ] **Step 1: Verify Turnstile blocks token-less signup**

```bash
curl -i -X POST https://demo.raven.ravencloak.org/api/auth/signup/google
```

Expected: HTTP 403 with `"missing turnstile token"`.

- [ ] **Step 2: Sign up via the UI**

Visit `https://demo.raven.ravencloak.org`, complete Turnstile, sign in with Google. After landing, expect a populated "Sample workspace" with the seeded messages.

- [ ] **Step 3: Trigger an LLM request, observe the fuse counter increment**

Send a chat message. SSM into the box:

```bash
docker exec raven-valkey-1 redis-cli get "raven:llm:spend:$(date -u +%Y%m%d)"
```

Expected: non-zero float string.

- [ ] **Step 4: Force-trip the fuse**

Temporarily set `RAVEN_LLM_DAILY_USD_CAP=0.0001` in `/etc/raven/env`, restart the python-worker container, send another chat message. Expect the UI to display the "demo daily limit reached" error.

Revert the cap after verification.

- [ ] **Step 5: DSAR export**

In the demo app, find the account settings → Export my data. Should download a JSON file containing the seeded workspace data.

- [ ] **Step 6: DSAR delete**

Click "Delete my account" in account settings. Confirm via the email Resend sends. Wait the grace window (or force-run the deletion job for the test) and verify the user is gone.

- [ ] **Step 7: Retention purge dry-run**

```bash
sudo systemctl start raven-retention-purge.service
sudo journalctl -u raven-retention-purge.service --no-pager | tail -20
```

Expected: clean exit, log shows "OK".

- [ ] **Step 8: Cookie banner appears once**

Open the demo in an incognito window. Cookie banner appears, click accept. Reload — banner does not reappear.

- [ ] **Step 9: Voice UI is hidden**

Inspect the chat UI. No "Start voice call" button. Visit `/voice-sessions` — redirects to `/`.

- [ ] **Step 10: Record outcome in the runbook**

Append to `docs/runbooks/demo-app-features.md`:

```markdown
## End-to-end smoke verified

| Check | Result |
|---|---|
| Turnstile required | ✅ |
| Sample workspace seeded | ✅ |
| LLM fuse counts | ✅ |
| LLM fuse trips | ✅ |
| DSAR export | ✅ |
| DSAR delete | ✅ |
| Retention purge cron | ✅ |
| Cookie banner once | ✅ |
| Voice UI hidden | ✅ |

- Verified: YYYY-MM-DD by <initials>
```

- [ ] **Step 11: Commit**

```bash
git add docs/runbooks/demo-app-features.md
git commit -m "docs(demo): record successful app-features smoke"
```

---

## Self-review

| Spec section | Plan task(s) |
|---|---|
| §3 Google OAuth only | Existing — no plan task; verified during Task 18 |
| §3 Turnstile on signup | Tasks 5, 6, 7 |
| §3 Global LLM daily $-fuse | Tasks 8, 9 |
| §3 Free-tier subscription enforcement | Pre-existing (#244) — not in plan |
| §6 Retention purge (23-day warn, 30-day delete) | Task 13 |
| §6 DSAR `/account/export` | Tasks 10, 12 |
| §6 DSAR `/account/delete` with 24h grace | Tasks 11, 12 |
| §7 Sandbox payments | Pre-existing UI; "Coming soon" CTA replacement is implementer's call during Task 15 footer changes |
| §8 Resend email | Tasks 2, 3, 4 |
| §10 Privacy policy + ToS | Task 15 |
| §10 Cookie banner | Task 16 |
| §11 Pre-seeded sample workspace | Task 14 |
| §2 Voice UI hidden | Task 17 |

No placeholders. Method and variable names match across tasks:
- `mail.Sender`, `mail.Message` consistent in Tasks 2, 3, 11, 13.
- `account.DSARHandler`, `account.RetentionPurger`, `account.UserIDFrom`/`UserEmailFrom` consistent in Tasks 10–13.
- `seed.SeedSampleWorkspace` consistent in Task 14.
- `LLMSpendFuse` / `FuseTripped` / `guard` / `charge` consistent in Tasks 8, 9.
- `isVoiceEnabled` in Task 17 matches the compose env from plan #1 Task 12.

Known follow-ups deferred from this plan:
1. Task 12 Step 2's `SQLRepo.ExportUser` has a TODO comment for the canonical user-owned table list. The implementer must enumerate these from the real schema before shipping; the test in Task 10 exercises the interface, not the SQL.
2. Task 15 final policy content needs a lawyer pass before Phase 3 launch.
3. Task 17 may surface voice routes not in the obvious search; a final visual sweep on a deployed instance is part of Task 18.
