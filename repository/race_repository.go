package repository

import (
	"context"
	"errors"
	"time"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// raceEndGrace delays "race is finished" by a day. race_end_date is stored as
// midnight of race day (Sunday 00:00), so without this a race was marked
// COMPLETED (and scraped) many hours before it had even started.
const raceEndGrace = 24 * time.Hour

type RaceRepository struct {
	db *gorm.DB
}

func NewRaceRepository(db *gorm.DB) *RaceRepository {
	return &RaceRepository{db: db}
}

func (r *RaceRepository) FindAllRacesBySeasonName(ctx context.Context, seasonName string) ([]models.Race, error) {
	var races []models.Race
	err := DBFrom(ctx, r.db).Distinct().
		Preload("Status").
		Preload("RaceSeasons.Season").
		Joins("JOIN f1_api.race_season rs ON rs.race_id = f1_api.race.id").
		Joins("JOIN f1_api.season s ON s.id = rs.season_id").
		Where("s.season_name = ?", seasonName).
		Order("f1_api.race.race_start_date ASC NULLS LAST").
		Find(&races).Error
	return races, err
}

func (r *RaceRepository) FindByName(ctx context.Context, name string) (*models.Race, error) {
	var race models.Race
	err := DBFrom(ctx, r.db).
		Where("name = ?", name).
		First(&race).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &race, nil
}

func (r *RaceRepository) FindByNameAndRaceStartDateAndRaceEndDate(ctx context.Context, name string, startDate, endDate time.Time) (*models.Race, error) {
	var race models.Race
	err := DBFrom(ctx, r.db).
		Where("name = ? AND race_start_date = ? AND race_end_date = ?", name, startDate, endDate).
		First(&race).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &race, nil
}

// FindByRaceStartDateInSeason finds a race by its start date within one season.
// Used so a sponsor rename of the same race updates it instead of creating a
// duplicate that would never match an F1.com result page.
func (r *RaceRepository) FindByRaceStartDateInSeason(ctx context.Context, seasonName string, startDate time.Time) (*models.Race, error) {
	query := `
		SELECT r.* FROM f1_api.race r
		JOIN f1_api.race_season rsj ON rsj.race_id = r.id
		JOIN f1_api.season s ON s.id = rsj.season_id
		WHERE s.season_name = ?
		AND r.race_start_date = ?
		ORDER BY r.created_at ASC
		LIMIT 1`

	var races []models.Race
	if err := DBFrom(ctx, r.db).Raw(query, seasonName, startDate).Scan(&races).Error; err != nil {
		return nil, err
	}
	if len(races) == 0 {
		return nil, nil
	}
	return &races[0], nil
}

func (r *RaceRepository) FindByCountryIgnoreCase(ctx context.Context, country string) (*models.Race, error) {
	var race models.Race
	err := DBFrom(ctx, r.db).Where("LOWER(country) = LOWER(?)", country).First(&race).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &race, nil
}

func (r *RaceRepository) UpdateDetails(ctx context.Context, id uuid.UUID, name, country string, start, end time.Time) error {
	return DBFrom(ctx, r.db).Model(&models.Race{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":            name,
			"country":         country,
			"race_start_date": start,
			"race_end_date":   end,
			"updated_at":      time.Now(),
		}).Error
}

func (r *RaceRepository) Save(ctx context.Context, race *models.Race) error {
	return DBFrom(ctx, r.db).Save(race).Error
}

func (r *RaceRepository) MarkCancelledRaces(ctx context.Context, year string) error {
	query := `
		UPDATE f1_api.race
		SET status_id = (SELECT id FROM f1_api.race_status WHERE name = 'CANCELLED'),
			updated_at = NOW()
		WHERE id IN (
			SELECT r.id FROM f1_api.race r
			JOIN f1_api.race_season rsj ON r.id = rsj.race_id
			JOIN f1_api.season s ON rsj.season_id = s.id
			WHERE s.season_name = ?
			AND (
				r.name ILIKE 'CALLED OFF%'
				OR r.name ILIKE '%CANCELLED%'
				OR r.name ILIKE '%CANCELED%'
			)
		)`

	return DBFrom(ctx, r.db).Exec(query, year).Error
}

func (r *RaceRepository) UpdatePastScheduledRacesToCompleted(ctx context.Context, now time.Time, year string) error {
	query := `
		UPDATE f1_api.race
		SET status_id = (SELECT id FROM f1_api.race_status WHERE name = 'COMPLETED'),
			updated_at = NOW()
		WHERE id IN (
			SELECT r.id FROM f1_api.race r
			JOIN f1_api.race_status rs ON r.status_id = rs.id
			JOIN f1_api.race_season rsj ON r.id = rsj.race_id
			JOIN f1_api.season s ON rsj.season_id = s.id
			WHERE s.season_name = ?
			AND r.race_end_date < ?
			AND rs.name = 'SCHEDULED'
			AND r.name NOT ILIKE 'CALLED OFF%'
			AND r.name NOT ILIKE '%CANCELLED%'
			AND r.name NOT ILIKE '%CANCELED%'
		)`

	return DBFrom(ctx, r.db).Exec(query, year, now.Add(-raceEndGrace)).Error
}

func (r *RaceRepository) FindAllFinishedRacesMissingResults(ctx context.Context, now time.Time, year string) ([]models.Race, error) {
	query := `
		SELECT r.* FROM f1_api.race r
		JOIN f1_api.race_status rs ON r.status_id = rs.id
		JOIN f1_api.race_season rsj ON r.id = rsj.race_id
		JOIN f1_api.season s ON rsj.season_id = s.id
		WHERE s.season_name = ?
		AND r.race_end_date < ?
		AND rs.name = 'COMPLETED'
		AND r.name NOT ILIKE '%TESTING%'
		AND r.name NOT ILIKE '%PRE-SEASON%'
		AND r.name NOT ILIKE 'CALLED OFF%'
		AND r.name NOT ILIKE '%CANCELLED%'
		AND r.name NOT ILIKE '%CANCELED%'
		AND NOT EXISTS (
			SELECT 1 FROM f1_api.driver_race_result drr
			WHERE drr.race_id = r.id
		)
		ORDER BY r.race_end_date ASC`

	var races []models.Race
	err := DBFrom(ctx, r.db).Raw(query, year, now.Add(-raceEndGrace)).Find(&races).Error
	return races, err
}

func (r *RaceRepository) FindFirstFinishedRaceMissingResults(ctx context.Context, now time.Time, year string) (*models.Race, error) {
	races, err := r.FindAllFinishedRacesMissingResults(ctx, now, year)
	if err != nil || len(races) == 0 {
		return nil, err
	}
	return &races[0], nil
}

func (r *RaceRepository) FindCompletedNonSprintRaces(ctx context.Context, now time.Time, year string) ([]models.Race, error) {
	query := `
		SELECT r.* FROM f1_api.race r
		JOIN f1_api.race_status rs ON r.status_id = rs.id
		JOIN f1_api.race_season rsj ON r.id = rsj.race_id
		JOIN f1_api.season s ON rsj.season_id = s.id
		WHERE s.season_name = ?
		AND r.race_end_date < ?
		AND rs.name = 'COMPLETED'
		AND r.is_sprint = false
		AND r.name NOT ILIKE '%TESTING%'
		AND r.name NOT ILIKE '%PRE-SEASON%'
		AND r.name NOT ILIKE 'CALLED OFF%'
		AND r.name NOT ILIKE '%CANCELLED%'
		AND r.name NOT ILIKE '%CANCELED%'
		ORDER BY r.race_end_date ASC`

	var races []models.Race
	err := DBFrom(ctx, r.db).Raw(query, year, now.Add(-raceEndGrace)).Find(&races).Error
	return races, err
}
