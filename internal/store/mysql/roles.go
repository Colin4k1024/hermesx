package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
)

type myRoleStore struct{ db *sql.DB }

func (s *myRoleStore) Create(ctx context.Context, role *store.Role) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO roles (id, tenant_id, name, description, is_system)
		 VALUES (?, ?, ?, ?, ?)`,
		role.ID, role.TenantID, role.Name, nullStr(role.Description), role.IsSystem)
	return err
}

func (s *myRoleStore) Get(ctx context.Context, tenantID, roleID string) (*store.Role, error) {
	var r store.Role
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, COALESCE(description,''), is_system, created_at
		 FROM roles WHERE tenant_id = ? AND id = ?`, tenantID, roleID).
		Scan(&r.ID, &r.TenantID, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("role not found")
	}
	return &r, err
}

func (s *myRoleStore) GetByName(ctx context.Context, tenantID, name string) (*store.Role, error) {
	var r store.Role
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, COALESCE(description,''), is_system, created_at
		 FROM roles WHERE tenant_id = ? AND name = ?`, tenantID, name).
		Scan(&r.ID, &r.TenantID, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("role %q not found", name)
	}
	return &r, err
}

func (s *myRoleStore) List(ctx context.Context, tenantID string) ([]*store.Role, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, COALESCE(description,''), is_system, created_at
		 FROM roles WHERE tenant_id = ? ORDER BY name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*store.Role
	for rows.Next() {
		var r store.Role
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, &r)
	}
	return roles, rows.Err()
}

func (s *myRoleStore) Delete(ctx context.Context, tenantID, roleID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM roles WHERE tenant_id = ? AND id = ? AND is_system = 0`, tenantID, roleID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("role not found or is a system role")
	}
	return nil
}

func (s *myRoleStore) AddPermission(ctx context.Context, tenantID, roleName, resource, action string) error {
	permID := uuid.New().String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO role_permissions (id, role_id, resource, action)
		 SELECT ?, r.id, ?, ? FROM roles r WHERE r.tenant_id = ? AND r.name = ?
		 ON DUPLICATE KEY UPDATE resource = VALUES(resource)`,
		permID, resource, action, tenantID, roleName)
	return err
}

func (s *myRoleStore) RemovePermission(ctx context.Context, tenantID, roleName, resource, action string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE rp FROM role_permissions rp
		 JOIN roles r ON rp.role_id = r.id
		 WHERE r.tenant_id = ? AND r.name = ? AND rp.resource = ? AND rp.action = ?`,
		tenantID, roleName, resource, action)
	return err
}

func (s *myRoleStore) ListPermissions(ctx context.Context, tenantID, roleName string) ([]*store.RolePermission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT rp.id, rp.role_id, rp.resource, rp.action, rp.created_at
		 FROM role_permissions rp
		 JOIN roles r ON rp.role_id = r.id
		 WHERE r.tenant_id = ? AND r.name = ?
		 ORDER BY rp.resource, rp.action`, tenantID, roleName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []*store.RolePermission
	for rows.Next() {
		var p store.RolePermission
		if err := rows.Scan(&p.ID, &p.RoleID, &p.Resource, &p.Action, &p.CreatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, &p)
	}
	return perms, rows.Err()
}

func (s *myRoleStore) HasPermission(ctx context.Context, roles []string, tenantID, resource, action string) (bool, error) {
	if len(roles) == 0 {
		return false, nil
	}

	// Build IN clause with positional ? placeholders.
	placeholders := strings.Repeat("?,", len(roles)-1) + "?"
	args := make([]any, 0, 1+len(roles)+2)
	args = append(args, tenantID)
	for _, r := range roles {
		args = append(args, r)
	}
	args = append(args, resource, action)

	var exists bool
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(
		`SELECT EXISTS(
			SELECT 1 FROM role_permissions rp
			JOIN roles r ON rp.role_id = r.id
			WHERE r.tenant_id = ? AND r.name IN (%s)
			  AND rp.resource = ? AND rp.action = ?
		)`, placeholders), args...).Scan(&exists)
	return exists, err
}

var _ store.RoleStore = (*myRoleStore)(nil)
