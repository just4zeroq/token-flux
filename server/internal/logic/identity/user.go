package identity

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"ai-platform/internal/model/dto"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"golang.org/x/crypto/bcrypt"
)

func (s *sIdentity) Register(ctx context.Context, in dto.RegisterIn) (*dto.RegisterOut, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	count, err := g.DB().Model("users").Ctx(ctx).Where("email", in.Email).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "check email failed")
	}
	if count > 0 {
		return nil, gerror.New("email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, gerror.Wrap(err, "hash password failed")
	}

	username := strings.Split(in.Email, "@")[0]
	existCount, err := g.DB().Model("users").Ctx(ctx).Where("username", username).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "check username failed")
	}
	if existCount > 0 {
		username = fmt.Sprintf("%s_%d", username, time.Now().UnixNano()%100000)
	}

	_, err = g.DB().Model("users").Ctx(ctx).Data(g.Map{
		"username":   username,
		"email":      in.Email,
		"password":   string(hash),
		"source":     "email",
		"status":     0,
		"role":       dto.RoleUser,
		"created_at": gtime.Now(),
		"updated_at": gtime.Now(),
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create user failed")
	}

	code, err := generateCode(6)
	if err != nil {
		return nil, gerror.Wrap(err, "generate code failed")
	}

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

	g.Log().Infof(ctx, "[EMAIL] Verification code for %s: %s", in.Email, code)

	return &dto.RegisterOut{Message: "verification code sent to email"}, nil
}

func (s *sIdentity) VerifyEmail(ctx context.Context, in dto.VerifyEmailIn) (*dto.VerifyEmailOut, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

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

	if record["code"].String() != in.Code {
		return nil, gerror.New("invalid verification code")
	}

	_, err = g.DB().Model("email_verifications").Ctx(ctx).
		Where("id", record["id"].Int64()).
		Data("used_at", gtime.Now()).
		Update()
	if err != nil {
		return nil, gerror.Wrap(err, "mark code used failed")
	}

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

	if record["status"].Int() != 1 {
		return nil, gerror.New("account not activated — please verify your email first")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(record["password"].String()),
		[]byte(in.Password),
	); err != nil {
		return nil, gerror.New("invalid email or password")
	}

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

func generateCode(length int) (string, error) {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		result[i] = byte('0') + byte(n.Int64())
	}
	return string(result), nil
}
