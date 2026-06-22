package users

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListByOrgID(ctx context.Context, orgID string) ([]UserListItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, email, role, auth_provider, created_at FROM "user" WHERE org_id = $1 ORDER BY created_at ASC`,
		orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]UserListItem, 0)
	for rows.Next() {
		var u UserListItem
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.AuthProvider, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}
