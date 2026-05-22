# Identity Migration — Email Registration + Verification Code (P2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the identity domain: email registration with 6-digit verification code, bcrypt login, JWT auth middleware, API key CRUD + validation middleware. After this plan the API serves `POST /api/v1/auth/register`, `POST /api/v1/auth/verify-email`, `POST /api/v1/auth/login`, `GET /api/v1/users/me`, and API key endpoints — all with real database-backed auth.

**Architecture:**

- Registration: user submits email+password → user created (status=0, pending) → 6-digit code stored in `email_verifications` → code logged (email sending deferred to future SMTP integration) → user calls verify-email with code → status→1 (active) → can login.
- Login: email+password → bcrypt verify → JWT (HS256, 24h) returned. JWT carries `user_id`, `username`, `role`.
- JWT middleware: extract Bearer token from Authorization header, validate, inject `user_id`/`username`/`role` into `r.SetCtxVar`. Applied to all `/users/*` and `/auth/*` protected routes (except register/login/verify).
- API Key validation: extract Bearer token starting with `sk-`, lookup in `api_keys` table by plain text key, check `status=1` and not expired/deleted. Inject `user_id`/`api_key_id`.

**Tech Stack:** Go 1.26, GoFrame v2.10, `golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`.

---

## Scope

Pre-req: P1 complete — monolith skeleton compiles, both servers start, `/health` works.

In:

- Migration `0010_add_email_verification.sql` — `email_verifications` table + UNIQUE on `users.email`
- `internal/model/dto/identity.go` — request/response types
- `internal/service/identity.go` — IIdentity interface with 9 methods
- `internal/logic/identity/` — user.go (register/verify/login/profile), apikey.go (CRUD+validate), jwt.go (generate+validate)
- `internal/middleware/jwt.go` — JWT auth middleware
- `internal/middleware/apikey.go` — API Key auth middleware
- `internal/controller/api/identity/identity.go` — HTTP handlers
- `internal/boot/boot.go` — route registration update
- `manifest/config/config.yaml` — add email.verification_code.length + ttl (no change needed if we hardcode for now, but better to config-drive)

Out (later plans):

- Real email sending (SMTP / SES integration) — verification codes are logged to stdout for now
- Password reset / forgot-password flow
- OAuth / social login
- Phone registration
- Rate limiting on register/login/verify endpoints

---

## Task 1: Migration — email_verifications table

**Files:**

- Create: `server/migrations/0010_add_email_verification.sql`

- [ ] **Step 1: Write the migration**

The existing `users` table (from 0001) has `email VARCHAR(255) NOT NULL DEFAULT ''` without a UNIQUE constraint. We add the constraint and create the verification table.

```sql
-- +goose Up
-- Add UNIQUE constraint on users.email for email-based login.
-- Existing rows with empty email are unaffected (NULL != '' in UNIQUE semantics,
-- but DEFAULT '' means all pre-existing rows have the same empty string, which
-- would violate UNIQUE. We set them to a per-row temp value first, then add the
-- constraint, then revert. For safety just skip pre-existing empty rows by not
-- touching them — empty email users can't login by email anyway.)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'users_email_unique'
    ) THEN
        -- Set empty emails to NULL so UNIQUE doesn't conflict on '' duplicates
        UPDATE users SET email = NULL WHERE email = '';
        ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);
    END IF;
END $$;

CREATE TABLE email_verifications (
    id         BIGSERIAL PRIMARY KEY,
    email      VARCHAR(255) NOT NULL,
    code       VARCHAR(16)  NOT NULL,
    purpose    VARCHAR(32)  NOT NULL DEFAULT 'register',   -- register | login | reset_password
    expires_at TIMESTAMPTZ  NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_verifications_email_purpose ON email_verifications(email, purpose, used_at, expires_at);

-- +goose Down
DROP TABLE IF EXISTS email_verifications;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_unique;
```

- [ ] **Step 2: Commit**

```bash
git add server/migrations/0010_add_email_verification.sql
git commit -m "feat(identity): add email_verifications table + email UNIQUE constraint"
```

---

## Task 2: DTO types

**Files:**

- Create: `server/internal/model/dto/identity.go`

- [ ] **Step 1: Write DTO types**

```go
package dto

import "time"

// ========== Registration ==========

type RegisterIn struct {
	Email    string `json:"email" v:"required|email|length:1,255"`
	Password string `json:"password" v:"required|length:8,128"`
}

type RegisterOut struct {
	Message string `json:"message"`
}

type VerifyEmailIn struct {
	Email string `json:"email" v:"required|email"`
	Code  string `json:"code" v:"required|length:6,6"`
}

type VerifyEmailOut struct {
	Message string `json:"message"`
}

// ========== Login ==========

type LoginIn struct {
	Email    string `json:"email" v:"required|email"`
	Password string `json:"password" v:"required"`
}

type LoginOut struct {
	Token string  `json:"token"`
	User  UserInfo `json:"user"`
}

// ========== User ==========

type UserInfo struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
	Role        int    `json:"role"`
	KYCStatus   string `json:"kyc_status"`
}

// ========== JWT / Auth ==========

type TokenClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     int    `json:"role"`
}

// ========== API Key ==========

type CreateApiKeyIn struct {
	Name          string `json:"name" v:"required|length:1,64"`
	QuotaCredits  *int64 `json:"quota_credits"`
	UnlimitedQuota bool  `json:"unlimited_quota"`
}

type ApiKeyOut struct {
	ID        int64      `json:"id"`
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	Status    int        `json:"status"`
	ExpireAt  *time.Time `json:"expire_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type ApiKeyInfo struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	Name             string     `json:"name"`
	Status           int        `json:"status"`
	QuotaCredits     *int64     `json:"quota_credits,omitempty"`
	RemainQuota      int64      `json:"remain_quota"`
	UsedQuota        int64      `json:"used_quota"`
	UnlimitedQuota   bool       `json:"unlimited_quota"`
	ModelLimits      string     `json:"model_limits"`
	AllowIPs         string     `json:"allow_ips"`
	ExpireAt         *time.Time `json:"expire_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type DeleteApiKeyIn struct {
	KeyID int64 `json:"key_id" v:"required"`
}
```

- [ ] **Step 2: Commit**

```bash
git add server/internal/model/dto/identity.go
git commit -m "feat(identity): add DTO types for register/login/keys"
```

---

## Task 3: Service interface

**Files:**

- Modify: `server/internal/service/identity.go`

- [ ] **Step 1: Update IIdentity interface**

```go
package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

// IIdentity is the contract for the identity domain (users, JWT, API keys).
type IIdentity interface {
	// Auth
	Register(ctx context.Context, in dto.RegisterIn) (*dto.RegisterOut, error)
	VerifyEmail(ctx context.Context, in dto.VerifyEmailIn) (*dto.VerifyEmailOut, error)
	Login(ctx context.Context, in dto.LoginIn) (*dto.LoginOut, error)

	// User
	GetUser(ctx context.Context, userID int64) (*dto.UserInfo, error)

	// Token
	ValidateToken(ctx context.Context, token string) (*dto.TokenClaims, error)

	// API Key
	ValidateApiKey(ctx context.Context, key string) (*dto.ApiKeyInfo, error)
	CreateApiKey(ctx context.Context, userID int64, in dto.CreateApiKeyIn) (*dto.ApiKeyOut, error)
	ListApiKeys(ctx context.Context, userID int64) ([]*dto.ApiKeyInfo, error)
	DeleteApiKey(ctx context.Context, userID, keyID int64) error
}

var localIdentity IIdentity

func RegisterIdentity(i IIdentity) { localIdentity = i }

func Identity() IIdentity {
	if localIdentity == nil {
		panic("service.Identity not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localIdentity
}
```

- [ ] **Step 2: Verify compile**

Run: `cd server && go build ./internal/service/`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add server/internal/service/identity.go
git commit -m "feat(identity): define IIdentity interface with 9 methods"
```

---

## Task 4: Logic — user registration, email verification, login

**Files:**

- Create: `server/internal/logic/identity/user.go`
- Create: `server/internal/logic/identity/jwt.go`
- Modify: `server/internal/logic/identity/identity.go`

- [ ] **Step 1: Write jwt.go — JWT helpers**

```go
package identity

import (
	"time"

	"ai-platform/internal/model/dto"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type jwtClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     int    `json:"role"`
	jwtv5.RegisteredClaims
}

func generateJWT(secret []byte, expireHours int, c dto.TokenClaims) (string, error) {
	claims := &jwtClaims{
		UserID:   c.UserID,
		Username: c.Username,
		Role:     c.Role,
		RegisteredClaims: jwtv5.RegisteredClaims{
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwtv5.NewNumericDate(time.Now()),
		},
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func parseJWT(secret []byte, tokenStr string) (*dto.TokenClaims, error) {
	token, err := jwtv5.ParseWithClaims(tokenStr, &jwtClaims{},
		func(t *jwtv5.Token) (any, error) { return secret, nil },
	)
	if err != nil || !token.Valid {
		return nil, err
	}
	claims, ok := token.Claims.(*jwtClaims)
	if !ok {
		return nil, jwtv5.ErrSignatureInvalid
	}
	return &dto.TokenClaims{
		UserID:   claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}
```

- [ ] **Step 2: Write user.go — Registration, verification, login, profile**

```go
package identity

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"ai-platform/internal/model/dto"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"golang.org/x/crypto/bcrypt"
)

func (s *sIdentity) Register(ctx context.Context, in dto.RegisterIn) (*dto.RegisterOut, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	// Check email uniqueness
	count, err := g.DB().Model("users").Ctx(ctx).Where("email", in.Email).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "check email failed")
	}
	if count > 0 {
		return nil, gerror.New("email already registered")
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, gerror.Wrap(err, "hash password failed")
	}

	// Derive username from email prefix
	username := strings.Split(in.Email, "@")[0]
	// Ensure unique username by appending suffix if needed
	existCount, err := g.DB().Model("users").Ctx(ctx).Where("username", username).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "check username failed")
	}
	if existCount > 0 {
		username = fmt.Sprintf("%s_%d", username, time.Now().UnixNano()%100000)
	}

	// Create user (status=0 = pending email verification)
	_, err = g.DB().Model("users").Ctx(ctx).Data(g.Map{
		"username":  username,
		"email":     in.Email,
		"password":  string(hash),
		"source":    "email",
		"status":    0,
		"role":      1, // consumer
		"created_at": gtime.Now(),
		"updated_at": gtime.Now(),
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create user failed")
	}

	// Generate 6-digit verification code
	code, err := generateCode(6)
	if err != nil {
		return nil, gerror.Wrap(err, "generate code failed")
	}

	// Store verification code
	_, err = g.DB().Model("email_verifications").Ctx(ctx).Data(g.Map{
		"email":      in.Email,
		"code":       code,
		"purpose":    "register",
		"expires_at": gtime.Now().Add(10 * time.Minute),
		"created_at": gtime.Now(),
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "store verification code failed")
	}

	// TODO: send email — for now, log the code
	g.Log().Infof(ctx, "[EMAIL] Verification code for %s: %s", in.Email, code)

	return &dto.RegisterOut{Message: "verification code sent to email"}, nil
}

func (s *sIdentity) VerifyEmail(ctx context.Context, in dto.VerifyEmailIn) (*dto.VerifyEmailOut, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	// Find latest unused, unexpired code
	record, err := g.DB().Model("email_verifications").Ctx(ctx).
		Where("email", in.Email).
		Where("purpose", "register").
		WhereNull("used_at").
		WhereGTE("expires_at", gtime.Now()).
		OrderDesc("id").
		Limit(1).
		One()
	if err != nil {
		return nil, gerror.Wrap(err, "query verification code failed")
	}
	if record.IsEmpty() {
		return nil, gerror.New("verification code not found or expired")
	}

	// Check code match
	if record["code"].String() != in.Code {
		return nil, gerror.New("invalid verification code")
	}

	// Mark code as used
	_, err = g.DB().Model("email_verifications").Ctx(ctx).
		Where("id", record["id"].Int64()).
		Data("used_at", gtime.Now()).
		Update()
	if err != nil {
		return nil, gerror.Wrap(err, "mark code used failed")
	}

	// Activate user
	_, err = g.DB().Model("users").Ctx(ctx).
		Where("email", in.Email).
		Data(g.Map{"status": 1, "updated_at": gtime.Now()}).
		Update()
	if err != nil {
		return nil, gerror.Wrap(err, "activate user failed")
	}

	return &dto.VerifyEmailOut{Message: "email verified successfully"}, nil
}

func (s *sIdentity) Login(ctx context.Context, in dto.LoginIn) (*dto.LoginOut, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	record, err := g.DB().Model("users").Ctx(ctx).Where("email", in.Email).One()
	if err != nil {
		return nil, gerror.Wrap(err, "query user failed")
	}
	if record.IsEmpty() {
		return nil, gerror.New("invalid email or password")
	}

	// Check user status
	if record["status"].Int() != 1 {
		return nil, gerror.New("account not activated — please verify your email first")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword(
		[]byte(record["password"].String()),
		[]byte(in.Password),
	); err != nil {
		return nil, gerror.New("invalid email or password")
	}

	// Generate JWT
	jwtSecret := g.Cfg().MustGet(ctx, "jwt.secret").String()
	jwtExpireHours := g.Cfg().MustGet(ctx, "jwt.expireHours", 24).Int()

	token, err := generateJWT([]byte(jwtSecret), jwtExpireHours, dto.TokenClaims{
		UserID:   record["id"].Int64(),
		Username: record["username"].String(),
		Role:     record["role"].Int(),
	})
	if err != nil {
		return nil, gerror.Wrap(err, "generate token failed")
	}

	return &dto.LoginOut{
		Token: token,
		User: dto.UserInfo{
			ID:          record["id"].Int64(),
			Username:    record["username"].String(),
			Email:       record["email"].String(),
			DisplayName: record["display_name"].String(),
			Avatar:      record["avatar"].String(),
			Role:        record["role"].Int(),
			KYCStatus:   record["kyc_status"].String(),
		},
	}, nil
}

func (s *sIdentity) GetUser(ctx context.Context, userID int64) (*dto.UserInfo, error) {
	record, err := g.DB().Model("users").Ctx(ctx).Where("id", userID).One()
	if err != nil {
		return nil, gerror.Wrap(err, "query user failed")
	}
	if record.IsEmpty() {
		return nil, gerror.New("user not found")
	}
	return &dto.UserInfo{
		ID:          record["id"].Int64(),
		Username:    record["username"].String(),
		Email:       record["email"].String(),
		DisplayName: record["display_name"].String(),
		Avatar:      record["avatar"].String(),
		Role:        record["role"].Int(),
		KYCStatus:   record["kyc_status"].String(),
	}, nil
}

func (s *sIdentity) ValidateToken(ctx context.Context, token string) (*dto.TokenClaims, error) {
	jwtSecret := g.Cfg().MustGet(ctx, "jwt.secret").String()
	return parseJWT([]byte(jwtSecret), token)
}

// generateCode returns a numeric code of the given length.
func generateCode(length int) (string, error) {
	code := ""
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += n.String()
	}
	return code, nil
}

// txCreateUser creates user within an existing transaction (unused for now;
// registration doesn't need a tx since there's only one write. Kept for
// future scenarios like inviting a user + creating initial accounts.)
var _ gdb.TX // reference gdb types
```

- [ ] **Step 3: Write apikey.go — API Key CRUD + validation**

```go
package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"ai-platform/internal/model/dto"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func (s *sIdentity) CreateApiKey(ctx context.Context, userID int64, in dto.CreateApiKeyIn) (*dto.ApiKeyOut, error) {
	key := "sk-" + generateAPIKeySuffix()

	data := g.Map{
		"user_id":     userID,
		"key":         key,
		"name":        in.Name,
		"status":      1,
		"created_at":  gtime.Now(),
		"updated_at":  gtime.Now(),
	}
	if in.QuotaCredits != nil {
		data["quota_credits"] = *in.QuotaCredits
		data["remain_quota"] = *in.QuotaCredits
	}
	if in.UnlimitedQuota {
		data["unlimited_quota"] = true
	}

	result, err := g.DB().Model("api_keys").Ctx(ctx).Data(data).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create api key failed")
	}
	id, _ := result.LastInsertId()

	return &dto.ApiKeyOut{
		ID:        id,
		Key:       key,
		Name:      in.Name,
		Status:    1,
		CreatedAt: time.Now(),
	}, nil
}

func (s *sIdentity) ListApiKeys(ctx context.Context, userID int64) ([]*dto.ApiKeyInfo, error) {
	records, err := g.DB().Model("api_keys").Ctx(ctx).
		Where("user_id", userID).
		WhereNull("deleted_at").
		OrderDesc("id").
		All()
	if err != nil {
		return nil, gerror.Wrap(err, "list api keys failed")
	}

	out := make([]*dto.ApiKeyInfo, 0, len(records))
	for _, r := range records {
		info := &dto.ApiKeyInfo{
			ID:             r["id"].Int64(),
			UserID:         r["user_id"].Int64(),
			Name:           r["name"].String(),
			Status:         r["status"].Int(),
			RemainQuota:    r["remain_quota"].Int64(),
			UsedQuota:      r["used_quota"].Int64(),
			UnlimitedQuota: r["unlimited_quota"].Bool(),
			CreatedAt:      r["created_at"].Time(),
		}
		if v := r["quota_credits"]; !v.IsNil() {
			q := v.Int64()
			info.QuotaCredits = &q
		}
		if v := r["expire_time"]; !v.IsNil() {
			t := v.Time()
			info.ExpireAt = &t
		}
		out = append(out, info)
	}
	return out, nil
}

func (s *sIdentity) DeleteApiKey(ctx context.Context, userID, keyID int64) error {
	_, err := g.DB().Model("api_keys").Ctx(ctx).
		Where("id", keyID).
		Where("user_id", userID).
		WhereNull("deleted_at").
		Data(g.Map{
			"deleted_at": gtime.Now(),
			"updated_at": gtime.Now(),
		}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "delete api key failed")
	}
	return nil
}

func (s *sIdentity) ValidateApiKey(ctx context.Context, key string) (*dto.ApiKeyInfo, error) {
	if len(key) < 4 || key[:3] != "sk-" {
		return nil, gerror.New("invalid api key format")
	}

	record, err := g.DB().Model("api_keys").Ctx(ctx).
		Where("key", key).
		Where("status", 1).
		WhereNull("deleted_at").
		One()
	if err != nil {
		return nil, gerror.Wrap(err, "query api key failed")
	}
	if record.IsEmpty() {
		return nil, gerror.New("api key not found or revoked")
	}

	// Check expiry
	if expireAt := record["expire_time"]; !expireAt.IsNil() {
		if expireAt.Time().Before(time.Now()) {
			return nil, gerror.New("api key expired")
		}
	}

	return &dto.ApiKeyInfo{
		ID:             record["id"].Int64(),
		UserID:         record["user_id"].Int64(),
		Name:           record["name"].String(),
		Status:         record["status"].Int(),
		RemainQuota:    record["remain_quota"].Int64(),
		UsedQuota:      record["used_quota"].Int64(),
		UnlimitedQuota: record["unlimited_quota"].Bool(),
	}, nil
}

func generateAPIKeySuffix() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 4: Verify compile**

Run: `cd server && go build ./internal/logic/identity/`
Expected: no output, exit 0.

Fixes if needed:
- If `gtime.Now()` doesn't exist, use `time.Now()`.
- If GoFrame v2.10 DB record access pattern differs, adjust `.String()` / `.Int64()` calls.

- [ ] **Step 5: Commit**

```bash
git add server/internal/logic/identity/user.go server/internal/logic/identity/jwt.go server/internal/logic/identity/apikey.go
git commit -m "feat(identity): implement register/verify/login + JWT + API key logic"
```

---

## Task 5: Middleware — JWT + API Key

**Files:**

- Create: `server/internal/middleware/jwt.go`
- Create: `server/internal/middleware/apikey.go`

- [ ] **Step 1: Write jwt.go**

```go
package middleware

import (
	"strings"

	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
)

const CtxKeyUserID = "user_id"
const CtxKeyUsername = "username"
const CtxKeyRole = "role"

// JWTAuth validates the Bearer JWT token and injects user_id/username/role into
// the request context. Returns 401 if the token is missing, expired, or invalid.
func JWTAuth(r *ghttp.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		r.Response.WriteStatusExit(401, ghttp.DefaultHandlerResponse{
			Code:    401,
			Message: "missing authorization header",
		})
	}
	token := strings.TrimPrefix(auth, "Bearer ")

	claims, err := service.Identity().ValidateToken(r.Context(), token)
	if err != nil {
		r.Response.WriteStatusExit(401, ghttp.DefaultHandlerResponse{
			Code:    401,
			Message: "invalid or expired token",
		})
		return
	}

	r.SetCtxVar(CtxKeyUserID, claims.UserID)
	r.SetCtxVar(CtxKeyUsername, claims.Username)
	r.SetCtxVar(CtxKeyRole, claims.Role)
	r.Middleware.Next()
}

// GetUserID extracts the authenticated user ID from context.
// Panics if the context variable is missing (middleware bug).
func GetUserID(ctx context.Context) int64 {
	v := ctx.Value(CtxKeyUserID)
	if v == nil {
		panic("GetUserID: missing user_id in context — JWTAuth middleware not applied")
	}
	return v.(int64)
}

// GetUsername extracts the authenticated username from context.
func GetUsername(ctx context.Context) string {
	return ctx.Value(CtxKeyUsername).(string)
}
```

Note: The `GetUserID` function above uses `context.Context` — it needs `import "context"`. The actual implementation retrieves from `ghttp.Request` context via `r.GetCtxVar`. Let's adjust:

```go
package middleware

import (
	"strings"

	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

const CtxKeyUserID = "user_id"
const CtxKeyUsername = "username"
const CtxKeyRole = "role"

// JWTAuth validates the Bearer JWT token and injects user_id/username/role into
// the request context. Returns 401 if the token is missing, expired, or invalid.
func JWTAuth(r *ghttp.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		r.Response.WriteStatusExit(401, map[string]any{
			"code":    401,
			"message": "missing authorization header",
		})
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return // WriteStatusExit already handled the empty case
	}

	claims, err := service.Identity().ValidateToken(r.Context(), token)
	if err != nil {
		r.Response.WriteStatusExit(401, map[string]any{
			"code":    401,
			"message": "invalid or expired token",
		})
		return
	}

	r.SetCtxVar(CtxKeyUserID, claims.UserID)
	r.SetCtxVar(CtxKeyUsername, claims.Username)
	r.SetCtxVar(CtxKeyRole, claims.Role)
	r.Middleware.Next()
}

// GetUserID extracts the authenticated user ID from the request.
func GetUserID(r *ghttp.Request) int64 {
	return r.GetCtxVar(CtxKeyUserID).Int64()
}
```

- [ ] **Step 2: Write apikey.go**

```go
package middleware

import (
	"strings"

	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

const CtxKeyAPIKeyID = "api_key_id"

// APIKeyAuth validates the Bearer API key (sk-xxx) and injects user_id/api_key_id
// into the request context. Used on the gateway (:8081) routes.
func APIKeyAuth(r *ghttp.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		r.Response.WriteStatusExit(401, map[string]any{
			"code":    401,
			"message": "missing api key",
		})
	}
	key := strings.TrimPrefix(auth, "Bearer ")
	if key == "" {
		return
	}

	info, err := service.Identity().ValidateApiKey(r.Context(), key)
	if err != nil {
		r.Response.WriteStatusExit(401, map[string]any{
			"code":    401,
			"message": err.Error(),
		})
		return
	}

	r.SetCtxVar(CtxKeyUserID, info.UserID)
	r.SetCtxVar(CtxKeyAPIKeyID, info.ID)
	r.Middleware.Next()
}
```

- [ ] **Step 3: Verify compile**

Run: `cd server && go build ./internal/middleware/`
Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add server/internal/middleware/jwt.go server/internal/middleware/apikey.go
git commit -m "feat(identity): add JWT and API Key auth middleware"
```

---

## Task 6: Controller — HTTP handlers

**Files:**

- Create: `server/internal/controller/api/identity/identity.go`

- [ ] **Step 1: Write controller**

```go
package identity

import (
	"ai-platform/internal/middleware"
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// Register handles POST /api/v1/auth/register
func Register(r *ghttp.Request) {
	var in dto.RegisterIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
	}
	out, err := service.Identity().Register(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

// VerifyEmail handles POST /api/v1/auth/verify-email
func VerifyEmail(r *ghttp.Request) {
	var in dto.VerifyEmailIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
	}
	out, err := service.Identity().VerifyEmail(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

// Login handles POST /api/v1/auth/login
func Login(r *ghttp.Request) {
	var in dto.LoginIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
	}
	out, err := service.Identity().Login(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

// Me handles GET /api/v1/users/me (JWT required)
func Me(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	user, err := service.Identity().GetUser(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(user)
}

// CreateKey handles POST /api/v1/users/me/keys (JWT required)
func CreateKey(r *ghttp.Request) {
	var in dto.CreateApiKeyIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
	}
	userID := middleware.GetUserID(r)
	out, err := service.Identity().CreateApiKey(r.Context(), userID, in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

// ListKeys handles GET /api/v1/users/me/keys (JWT required)
func ListKeys(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	keys, err := service.Identity().ListApiKeys(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(keys)
}

// DeleteKey handles DELETE /api/v1/users/me/keys/{id} (JWT required)
func DeleteKey(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	keyID := r.Get("id").Int64()
	if keyID == 0 {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid key id"})
		return
	}
	if err := service.Identity().DeleteApiKey(r.Context(), userID, keyID); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"ok": true})
}
```

- [ ] **Step 2: Verify compile**

Run: `cd server && go build ./internal/controller/api/identity/`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add server/internal/controller/api/identity/identity.go
git commit -m "feat(identity): add HTTP controllers for auth + user + keys"
```

---

## Task 7: Route registration in boot.go

**Files:**

- Modify: `server/internal/boot/boot.go`

- [ ] **Step 1: Add identity routes to api server**

Replace the existing `boot.go` with the expanded version that registers all identity routes.

Before:
```go
apiSrv.Group("/", func(group *ghttp.RouterGroup) {
    group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
    group.GET("/health", health)
})
```

After:
```go
apiSrv.Group("/api/v1", func(group *ghttp.RouterGroup) {
    group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)

    // Public auth routes
    group.POST("/auth/register", apiIdentity.Register)
    group.POST("/auth/verify-email", apiIdentity.VerifyEmail)
    group.POST("/auth/login", apiIdentity.Login)
    group.GET ("/health", health) // keep health check

    // Protected routes (JWT required)
    group.Group("/", func(g *ghttp.RouterGroup) {
        g.Middleware(middleware.JWTAuth)

        g.GET ("/users/me", apiIdentity.Me)
        g.POST("/users/me/keys", apiIdentity.CreateKey)
        g.GET ("/users/me/keys", apiIdentity.ListKeys)
        g.DELETE("/users/me/keys/{id}", apiIdentity.DeleteKey)
    })
})
```

Full file:

```go
// Package boot wires together the HTTP servers and starts the application.
package boot

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	"ai-platform/internal/controller/api/identity"
	"ai-platform/internal/middleware"
)

// RunAPI starts both the api (:8080) and gateway (:8081) ghttp.Servers and blocks.
// All domain logic is registered via service interfaces during package init();
// this function only wires routes and starts servers.
func RunAPI() {
	ctx := gctx.New()

	apiSrv := g.Server("api")
	apiSrv.SetAddr(":8080")
	apiSrv.Group("/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)

		// Health
		group.GET("/health", health)

		// Public auth
		group.POST("/auth/register", identity.Register)
		group.POST("/auth/verify-email", identity.VerifyEmail)
		group.POST("/auth/login", identity.Login)

		// Protected (JWT required)
		group.Group("/", func(g *ghttp.RouterGroup) {
			g.Middleware(middleware.JWTAuth)
			g.GET("/users/me", identity.Me)
			g.POST("/users/me/keys", identity.CreateKey)
			g.GET("/users/me/keys", identity.ListKeys)
			g.DELETE("/users/me/keys/{id}", identity.DeleteKey)
		})
	})

	gwSrv := g.Server("gateway")
	gwSrv.SetAddr(":8081")
	gwSrv.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
		group.GET("/health", health)
	})

	if err := apiSrv.Start(); err != nil {
		g.Log().Fatalf(ctx, "api server start failed: %v", err)
	}
	if err := gwSrv.Start(); err != nil {
		g.Log().Fatalf(ctx, "gateway server start failed: %v", err)
	}

	g.Log().Info(ctx, "ai-platform started: api :8080 + gateway :8081")
	g.Wait()
}

func health(r *ghttp.Request) {
	r.Response.WriteJson(g.Map{"status": "ok"})
}
```

- [ ] **Step 2: Update imports**

Note: The import must reference `"ai-platform/internal/controller/api/identity"` — the package name `identity` conflicts with no other `identity` import, but the local alias is `identity` (from the package name). Make sure the import path and usage match.

- [ ] **Step 3: Verify compile**

Run: `cd server && go build ./internal/boot/`
Expected: no output, exit 0.

If compile fails due to unused import `"ai-platform/internal/middleware"`: it's already used by JWTAuth in the group middleware — should be fine.

- [ ] **Step 4: Commit**

```bash
git add server/internal/boot/boot.go
git commit -m "feat(identity): register auth + user + keys routes in boot"
```

---

## Task 8: Verification — build, register, verify, login

Pre-requisite: PostgreSQL running with `ai_platform` database, migrations applied.

- [ ] **Step 1: Full build**

Run: `cd server && go build ./...`
Expected: no output, exit 0.

- [ ] **Step 2: Vet**

Run: `cd server && go vet ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Build binary**

Run: `cd server && go build -o /tmp/ai-platform-api.exe ./cmd/api`
Expected: binary created.

- [ ] **Step 4: Run migrations** (if DB is fresh)

```bash
goose -dir server/migrations postgres "postgres://aiplatform:aiplatform@localhost:5432/ai_platform?sslmode=disable" up
```

- [ ] **Step 5: Start server**

Run: `/tmp/ai-platform-api.exe &`
Wait 3 seconds.

- [ ] **Step 6: Test registration + verification + login flow**

```bash
# Register
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"securepass123"}'
# Expected: {"message":"verification code sent to email"}
# Check server logs for the 6-digit code

# Verify email (use code from log)
curl -s -X POST http://localhost:8080/api/v1/auth/verify-email \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","code":"123456"}'
# Expected: {"message":"email verified successfully"}

# Login
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"securepass123"}'
# Expected: {"token":"eyJ...","user":{...}}

# Save token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"securepass123"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# Get me
curl -s http://localhost:8080/api/v1/users/me -H "Authorization: Bearer $TOKEN"
# Expected: user info JSON

# Create API key
curl -s -X POST http://localhost:8080/api/v1/users/me/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-key"}'
# Expected: {"key":"sk-...","name":"my-key",...}

# List API keys
curl -s http://localhost:8080/api/v1/users/me/keys -H "Authorization: Bearer $TOKEN"
# Expected: array with 1 key

# Delete key
curl -s -X DELETE "http://localhost:8080/api/v1/users/me/keys/1" -H "Authorization: Bearer $TOKEN"
# Expected: {"ok":true}
```

- [ ] **Step 7: Test failure scenarios**

```bash
# Missing email
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"password":"securepass123"}'
# Expected: 400, validation error

# Wrong password
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"wrong"}'
# Expected: 400, "invalid email or password"

# No auth header
curl -s http://localhost:8080/api/v1/users/me
# Expected: 401, "missing authorization header"

# Wrong code
curl -s -X POST http://localhost:8080/api/v1/auth/verify-email \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","code":"000000"}'
# Expected: 400, "invalid verification code"
```

- [ ] **Step 8: Stop server**

Kill the background process.

- [ ] **Step 9: Commit any fixes**

If boot.go or any other file needed fixes during verification, commit them.

```bash
git commit -am "fix(identity): address issues found during verification"
```

P2 complete. The identity domain serves real database-backed email registration with verification code, JWT login, user profile, and API key CRUD. Next plan: P3 (billing/pricing/usage).