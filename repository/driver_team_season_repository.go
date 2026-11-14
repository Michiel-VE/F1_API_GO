package repository

import (
	"context"
	"errors"

	"f1-api-go/internal/dto"
	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DriverTeamSeasonRepository struct {
	db *gorm.DB
}

func NewDriverTeamSeasonRepository(db *gorm.DB) *DriverTeamSeasonRepository {
	return &DriverTeamSeasonRepository{db: db}
}

// FindByDriverAndSeasonAndTeam returns (nil, nil) when no row exists and an
// error only for real database failures (callers must not treat an error as
// "not found", or they will insert duplicates).
func (r *DriverTeamSeasonRepository) FindByDriverAndSeasonAndTeam(ctx context.Context, driverID, seasonID, teamID uuid.UUID) (*models.DriverTeamSeason, error) {
	var dts models.DriverTeamSeason
	err := DBFrom(ctx, r.db).
		Where("driver_id = ? AND season_id = ? AND team_id = ?", driverID, seasonID, teamID).
		First(&dts).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dts, nil
}

func (r *DriverTeamSeasonRepository) Save(ctx context.Context, dts *models.DriverTeamSeason) error {
	return DBFrom(ctx, r.db).Save(dts).Error
}

func (r *DriverTeamSeasonRepository) FindDriverPositionByLastNameAndSeason(ctx context.Context, driverLastName, seasonName string) (*int, error) {
	query := `
		WITH RankedResults AS (
			SELECT
				s.season_name,
				d.lastname AS driver_surname,
				dts.points,
				ROW_NUMBER() OVER (PARTITION BY s.id ORDER BY dts.points DESC) AS position
			FROM f1_api.driver d
			JOIN f1_api.driver_team_season dts ON dts.driver_id = d.id
			JOIN f1_api.season s ON dts.season_id = s.id
		)
		SELECT position
		FROM RankedResults
		WHERE LOWER(driver_surname) = LOWER(?) AND season_name = ?`

	var position int
	err := DBFrom(ctx, r.db).Raw(query, driverLastName, seasonName).Scan(&position).Error
	if err != nil {
		return nil, err
	}
	if position == 0 {
		return nil, nil
	}
	return &position, nil
}

func (r *DriverTeamSeasonRepository) FindDriverPositionByDtsID(ctx context.Context, dtsID uuid.UUID) (*int, error) {
	query := `
		WITH RankedResults AS (
			SELECT
				dts.id AS dts_id,
				ROW_NUMBER() OVER (PARTITION BY dts.season_id ORDER BY dts.points DESC) AS position
			FROM f1_api.driver_team_season dts
		)
		SELECT rr.position
		FROM RankedResults rr
		WHERE rr.dts_id = ?`

	var position int
	err := DBFrom(ctx, r.db).Raw(query, dtsID).Scan(&position).Error
	if err != nil {
		return nil, err
	}
	if position == 0 {
		return nil, nil
	}
	return &position, nil
}

func (r *DriverTeamSeasonRepository) FindAllTeamsBySeasonName(ctx context.Context, seasonName string) ([]dto.TeamWithPointsDTO, error) {
	query := `
		SELECT
			t.id,
			t.name,
			t.short_name,
			t.country,
			t.base,
			t.created_at,
			SUM(dts.points) AS total_points
		FROM f1_api.team t
		JOIN f1_api.driver_team_season dts ON t.id = dts.team_id
		JOIN f1_api.season s ON s.id = dts.season_id
		WHERE s.season_name = ?
		GROUP BY t.id, t.name, t.short_name, t.country, t.base, t.created_at
		ORDER BY total_points DESC`

	var results []dto.TeamWithPointsDTO
	err := DBFrom(ctx, r.db).Raw(query, seasonName).Scan(&results).Error
	return results, err
}

func (r *DriverTeamSeasonRepository) FindAllBySeasonID(ctx context.Context, seasonID uuid.UUID) ([]models.DriverTeamSeason, error) {
	var list []models.DriverTeamSeason
	err := DBFrom(ctx, r.db).Where("season_id = ?", seasonID).Find(&list).Error
	return list, err
}
