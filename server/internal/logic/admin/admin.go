package admin

import (
	"context"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

type sAdmin struct{}

func init() { service.RegisterAdmin(New()) }

// New returns the admin domain implementation.
func New() *sAdmin { return &sAdmin{} }

// ListUsers returns a paginated list of all users ordered by id descending.
func (s *sAdmin) ListUsers(ctx context.Context, page, pageSize int) ([]*dto.AdminUserInfo, int, error) {
	total, err := g.DB().Model("users").Ctx(ctx).Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count users failed")
	}
	if total == 0 {
		return []*dto.AdminUserInfo{}, 0, nil
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var users []*dto.AdminUserInfo
	err = g.DB().Model("users").Ctx(ctx).
		Order("id DESC").
		Offset(offset).Limit(pageSize).
		Scan(&users)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "list users failed")
	}
	return users, total, nil
}

// GetUser returns a single user by id.
func (s *sAdmin) GetUser(ctx context.Context, userID int64) (*dto.AdminUserInfo, error) {
	var user dto.AdminUserInfo
	err := g.DB().Model("users").Ctx(ctx).Where("id", userID).Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "query user failed")
	}
	if user.ID == 0 {
		return nil, gerror.New("user not found")
	}
	return &user, nil
}

// UpdateUserStatus updates the status column for a user.
func (s *sAdmin) UpdateUserStatus(ctx context.Context, in dto.UpdateUserStatusIn) error {
	result, err := g.DB().Model("users").Ctx(ctx).
		Where("id", in.UserID).
		Data(g.Map{"status": in.Status}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "update user status failed")
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return gerror.New("user not found")
	}
	return nil
}

// GetStats returns aggregate counts across users, api_keys, orders, and usage_records.
func (s *sAdmin) GetStats(ctx context.Context) (*dto.AdminStats, error) {
	totalUsers, err := g.DB().Model("users").Ctx(ctx).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "count users failed")
	}

	activeUsers, err := g.DB().Model("users").Ctx(ctx).Where("status", 1).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "count active users failed")
	}

	totalAPIKeys, err := g.DB().Model("api_keys").Ctx(ctx).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "count api_keys failed")
	}

	totalOrders, err := g.DB().Model("orders").Ctx(ctx).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "count orders failed")
	}

	totalUsageRecords, err := g.DB().Model("usage_records").Ctx(ctx).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "count usage_records failed")
	}

	return &dto.AdminStats{
		TotalUsers:        totalUsers,
		ActiveUsers:       activeUsers,
		TotalAPIKeys:      totalAPIKeys,
		TotalOrders:       totalOrders,
		TotalUsageRecords: totalUsageRecords,
	}, nil
}
