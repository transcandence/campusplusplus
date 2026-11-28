package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

type InventoryRepository struct {
	pool *pgxpool.Pool
}

func NewInventoryRepository(pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{pool: pool}
}

func (r *InventoryRepository) ListResidences(ctx context.Context, campusID *int64) ([]model.Residence, error) {
	const query = `
		SELECT id, campus_id, code, name, gender_policy::text
		FROM residences
		WHERE ($1::bigint IS NULL OR campus_id = $1)
		ORDER BY code`

	rows, err := r.pool.Query(ctx, query, campusID)
	if err != nil {
		return nil, fmt.Errorf("list residences: %w", err)
	}
	defer rows.Close()

	out := make([]model.Residence, 0)
	for rows.Next() {
		var res model.Residence
		if err := rows.Scan(&res.ID, &res.CampusID, &res.Code, &res.Name, &res.GenderPolicy); err != nil {
			return nil, fmt.Errorf("scan residence: %w", err)
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *InventoryRepository) ListDormUnits(ctx context.Context, f model.DormUnitFilter) ([]model.DormUnit, error) {
	const query = `
		SELECT du.id, du.floor_id, du.unit_number, du.code, du.unit_type::text, du.capacity
		FROM dorm_units du
		JOIN floors    f ON f.id = du.floor_id
		JOIN buildings b ON b.id = f.building_id
		WHERE ($1::bigint IS NULL OR b.residence_id     = $1)
		  AND ($2::bigint IS NULL OR du.floor_id        = $2)
		  AND ($3::text   IS NULL OR du.unit_type::text = $3)
		ORDER BY du.code`

	rows, err := r.pool.Query(ctx, query, f.ResidenceID, f.FloorID, f.UnitType)
	if err != nil {
		return nil, fmt.Errorf("list dorm units: %w", err)
	}
	defer rows.Close()

	out := make([]model.DormUnit, 0)
	for rows.Next() {
		var u model.DormUnit
		if err := rows.Scan(&u.ID, &u.FloorID, &u.UnitNumber, &u.Code, &u.UnitType, &u.Capacity); err != nil {
			return nil, fmt.Errorf("scan dorm unit: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
