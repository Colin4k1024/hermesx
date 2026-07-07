package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgAgentProfileStore struct{ pool *pgxpool.Pool }

func (s *pgAgentProfileStore) Create(ctx context.Context, p *store.AgentProfile) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	skillsJSON, err := json.Marshal(p.SelectedSkills)
	if err != nil {
		return fmt.Errorf("marshal selected_skills: %w", err)
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	return withTenantTx(ctx, s.pool, p.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO agent_profiles (id, tenant_id, user_id, name, description, model, selected_skills, is_default, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			p.ID, p.TenantID, p.UserID, p.Name, p.Description, p.Model, skillsJSON, p.IsDefault, p.CreatedAt, p.UpdatedAt)
		if err != nil {
			return fmt.Errorf("create agent profile: %w", err)
		}
		return nil
	})
}

func (s *pgAgentProfileStore) Get(ctx context.Context, tenantID, userID, profileID string) (*store.AgentProfile, error) {
	p := &store.AgentProfile{}
	var skillsJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, name, description, model, selected_skills, is_default, created_at, updated_at
		 FROM agent_profiles WHERE tenant_id = $1 AND user_id = $2 AND id = $3`, tenantID, userID, profileID).
		Scan(&p.ID, &p.TenantID, &p.UserID, &p.Name, &p.Description, &p.Model, &skillsJSON, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("get agent profile: %w", err)
	}
	if skillsJSON != nil {
		if err := json.Unmarshal(skillsJSON, &p.SelectedSkills); err != nil {
			return nil, fmt.Errorf("unmarshal selected_skills: %w", err)
		}
	}
	return p, nil
}

func (s *pgAgentProfileStore) List(ctx context.Context, tenantID, userID string) ([]*store.AgentProfile, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, name, description, model, selected_skills, is_default, created_at, updated_at
		 FROM agent_profiles WHERE tenant_id = $1 AND user_id = $2 ORDER BY created_at DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("list agent profiles: %w", err)
	}
	defer rows.Close()

	var profiles []*store.AgentProfile
	for rows.Next() {
		p := &store.AgentProfile{}
		var skillsJSON []byte
		if err := rows.Scan(&p.ID, &p.TenantID, &p.UserID, &p.Name, &p.Description, &p.Model, &skillsJSON, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent profile: %w", err)
		}
		if skillsJSON != nil {
			if err := json.Unmarshal(skillsJSON, &p.SelectedSkills); err != nil {
				return nil, fmt.Errorf("unmarshal selected_skills: %w", err)
			}
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (s *pgAgentProfileStore) Update(ctx context.Context, p *store.AgentProfile) error {
	skillsJSON, err := json.Marshal(p.SelectedSkills)
	if err != nil {
		return fmt.Errorf("marshal selected_skills: %w", err)
	}
	p.UpdatedAt = time.Now()

	return withTenantTx(ctx, s.pool, p.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE agent_profiles
			 SET name = $1, description = $2, model = $3, selected_skills = $4, is_default = $5, updated_at = $6
			 WHERE tenant_id = $7 AND user_id = $8 AND id = $9`,
			p.Name, p.Description, p.Model, skillsJSON, p.IsDefault, p.UpdatedAt, p.TenantID, p.UserID, p.ID)
		if err != nil {
			return fmt.Errorf("update agent profile: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}

func (s *pgAgentProfileStore) Delete(ctx context.Context, tenantID, userID, profileID string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM agent_profiles WHERE tenant_id = $1 AND user_id = $2 AND id = $3`,
			tenantID, userID, profileID)
		if err != nil {
			return fmt.Errorf("delete agent profile: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}

func (s *pgAgentProfileStore) GetDefault(ctx context.Context, tenantID, userID string) (*store.AgentProfile, error) {
	p := &store.AgentProfile{}
	var skillsJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, name, description, model, selected_skills, is_default, created_at, updated_at
		 FROM agent_profiles WHERE tenant_id = $1 AND user_id = $2 AND is_default = true`, tenantID, userID).
		Scan(&p.ID, &p.TenantID, &p.UserID, &p.Name, &p.Description, &p.Model, &skillsJSON, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("get default agent profile: %w", err)
	}
	if skillsJSON != nil {
		if err := json.Unmarshal(skillsJSON, &p.SelectedSkills); err != nil {
			return nil, fmt.Errorf("unmarshal selected_skills: %w", err)
		}
	}
	return p, nil
}

func (s *pgAgentProfileStore) SetDefault(ctx context.Context, tenantID, userID, profileID string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		// Clear existing default.
		if _, err := tx.Exec(ctx,
			`UPDATE agent_profiles SET is_default = false, updated_at = now()
			 WHERE tenant_id = $1 AND user_id = $2 AND is_default = true`, tenantID, userID); err != nil {
			return fmt.Errorf("clear default: %w", err)
		}
		// Set new default.
		tag, err := tx.Exec(ctx,
			`UPDATE agent_profiles SET is_default = true, updated_at = now()
			 WHERE tenant_id = $1 AND user_id = $2 AND id = $3`, tenantID, userID, profileID)
		if err != nil {
			return fmt.Errorf("set default: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}
