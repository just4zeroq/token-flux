package identity

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

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
			Status:      record["status"].Int(),
			KYCStatus:   record["kyc_status"].String(),
			CreatedAt:   record["created_at"].Time(),
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
		Status:      record["status"].Int(),
		KYCStatus:   record["kyc_status"].String(),
		CreatedAt:   record["created_at"].Time(),
	}, nil
}

func (s *sIdentity) ValidateToken(ctx context.Context, token string) (*dto.TokenClaims, error) {
	jwtSecret := g.Cfg().MustGet(ctx, "jwt.secret").String()
	return parseJWT([]byte(jwtSecret), token)
}

// ========== Admin ==========

func (s *sIdentity) ListUsers(ctx context.Context, page, pageSize int, status, role *int, keyword string) ([]*dto.UserInfo, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	model := g.DB().Model("users").Ctx(ctx)
	if status != nil {
		model = model.Where("status", *status)
	}
	if role != nil {
		model = model.Where("role", *role)
	}
	if keyword != "" {
		model = model.WhereLike("email", "%"+keyword+"%").WhereOrLike("username", "%"+keyword+"%")
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count users failed")
	}

	type userRow struct {
		ID          int64     `json:"id"`
		Username    string    `json:"username"`
		Email       string    `json:"email"`
		DisplayName string    `json:"display_name"`
		Avatar      string    `json:"avatar"`
		Role        int       `json:"role"`
		Status      int       `json:"status"`
		KYCStatus   string    `json:"kyc_status"`
		CreatedAt   time.Time `json:"created_at"`
	}

	var rows []userRow
	offset := (page - 1) * pageSize
	err = model.Order("id DESC").Limit(pageSize).Offset(offset).Scan(&rows)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query users failed")
	}

	list := make([]*dto.UserInfo, len(rows))
	for i, r := range rows {
		list[i] = &dto.UserInfo{
			ID:          r.ID,
			Username:    r.Username,
			Email:       r.Email,
			DisplayName: r.DisplayName,
			Avatar:      r.Avatar,
			Role:        r.Role,
			KYCStatus:   r.KYCStatus,
				Status:      r.Status,
				CreatedAt:   r.CreatedAt,
			}
	}

	return list, total, nil
}

func (s *sIdentity) CreateUser(ctx context.Context, in dto.CreateUserIn) (*dto.UserInfo, error) {
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
	existCount, _ := g.DB().Model("users").Ctx(ctx).Where("username", username).Count()
	if existCount > 0 {
		username = fmt.Sprintf("%s_%d", username, time.Now().UnixNano()%100000)
	}

	r, err := g.DB().Model("users").Ctx(ctx).Data(g.Map{
		"username":   username,
		"email":      in.Email,
		"password":   string(hash),
		"source":     "admin",
		"status":     1,
		"role":       in.Role,
		"created_at": gtime.Now(),
		"updated_at": gtime.Now(),
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create user failed")
	}

	id, _ := r.LastInsertId()
	return s.GetUser(ctx, id)
}

func (s *sIdentity) UpdateUserStatus(ctx context.Context, userID int64, status int) error {
	rows, err := g.DB().Model("users").Ctx(ctx).
		Where("id", userID).
		Data(g.Map{"status": status, "updated_at": gtime.Now()}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "update user status failed")
	}
	affected, _ := rows.RowsAffected()
	if affected == 0 {
		return gerror.New("user not found")
	}
	return nil
}

func (s *sIdentity) UpdateUserRole(ctx context.Context, userID int64, role int) error {
	rows, err := g.DB().Model("users").Ctx(ctx).
		Where("id", userID).
		Data(g.Map{"role": role, "updated_at": gtime.Now()}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "update user role failed")
	}
	affected, _ := rows.RowsAffected()
	if affected == 0 {
		return gerror.New("user not found")
	}
	return nil
}

func (s *sIdentity) ApplyProvider(ctx context.Context, userID int64, in dto.ProviderApplicationIn) error {
	if userID == 0 {
		return gerror.New("authentication required")
	}

	_, err := g.DB().Model("provider_applications").Ctx(ctx).Data(g.Map{
		"user_id":       userID,
		"company":       in.Company,
		"contact":       in.Contact,
		"email":         in.Email,
		"website":       in.Website,
		"bio":           in.Bio,
		"model_name":    in.ModelName,
		"model_family":  in.ModelFamily,
		"api_endpoint":  in.APIEndpoint,
		"documentation": in.Documentation,
		"status":        "pending",
	}).Insert()
	if err != nil {
		return gerror.Wrap(err, "submit provider application failed")
	}
	return nil
}

func (s *sIdentity) ListProviderApplications(ctx context.Context, page, pageSize int, status string) ([]*dto.ProviderApplicationInfo, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	m := g.DB().Model("provider_applications").Ctx(ctx).Order("id DESC")
	if status != "" {
		m = m.Where("status", status)
	}

	total, err := m.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count provider applications failed")
	}

	var rows []*dto.ProviderApplicationInfo
	if err := m.Page(page, pageSize).Scan(&rows); err != nil {
		return nil, 0, gerror.Wrap(err, "query provider applications failed")
	}
	return rows, total, nil
}

func (s *sIdentity) ReviewProviderApplication(ctx context.Context, appID int64, in dto.ReviewProviderApplicationIn) error {
	// Fetch application.
	var app dto.ProviderApplicationInfo
	if err := g.DB().Model("provider_applications").Ctx(ctx).Where("id", appID).Scan(&app); err != nil {
		return gerror.Wrap(err, "query application failed")
	}
	if app.ID == 0 {
		return gerror.New("application not found")
	}
	if app.Status != "pending" {
		return gerror.Newf("application already %s", app.Status)
	}

	// Update application status.
	if _, err := g.DB().Model("provider_applications").Ctx(ctx).
		Where("id", appID).
		Data(g.Map{
			"status":      in.Status,
			"review_note": in.ReviewNote,
			"reviewed_at": gtime.Now(),
		}).Update(); err != nil {
		return gerror.Wrap(err, "update application status failed")
	}

	// If approved: set user role to provider + set share_bps.
	if in.Status == "approved" {
		if err := s.UpdateUserRole(ctx, app.UserID, 1); err != nil {
			return gerror.Wrap(err, "set provider role failed")
		}
		if in.ShareBps > 0 {
			if err := service.LLM().SetProviderShareBps(ctx, app.UserID, in.ShareBps); err != nil {
				return gerror.Wrap(err, "set provider share failed")
			}
		}
	}
	return nil
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
