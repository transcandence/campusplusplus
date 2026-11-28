package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

func mapStayWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == "23505" && pgErr.ConstraintName == "uq_student_active_stay":
		return model.ErrDuplicateActiveStay
	case pgErr.Code == "23514":
		// trips ck_dorm_unit_capacity and the living-unit trigger
		return model.ErrUnitNotLiving
	}
	return err
}
