package repository

import (
	"errors"
	"time"

	"f1-api-go/internal/models"
	"gorm.io/gorm"
)

type RaceRepository struct {
	db *gorm.DB
}

func NewRaceRepository(db *gorm.DB) *RaceRepository {
	return &RaceRepository{db: db}
}

func (r *RaceRepository) FindAllRacesBySeasonName(seasonName string) ([]models.Race, error) {
	var races []models.Race
	err := r.db.Distinct().
		Preload("RaceSeasons.Season").
		Joins("JOIN f1_api.race_season rs ON rs.race_id = f1_api.race.id").
		Joins("JOIN f1_api.season s ON s.id = rs.season_id").
		Where("s.season_name = ?", seasonName).
		Order("f1_api.race.race_start_date ASC NULLS LAST").
		Find(&races).Error
	return races, err
}

func (r *RaceRepository) FindByNameAndRaceStartDateAndRaceEndDate(name string, startDate, endDate time.Time) (*models.Race, error) {
	var race models.Race
	err := r.db.Where("name = ? AND race_start_date = ? AND race_end_date = ?", name, startDate, endDate).First(&race).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &race, err
}

func (r *RaceRepository) FindByCountryIgnoreCase(country string) (*models.Race, error) {
	var race models.Race
	err := r.db.Where("LOWER(country) = LOWER(?)", country).First(&race).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &race, err
}

func (r *RaceRepository) FindFirstFinishedRaceMissingResults(now time.Time, year string) (*models.Race, error) {
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
		AND NOT EXISTS (
			SELECT 1 FROM f1_api.driver_race_result drr
			WHERE drr.race_id = r.id
		)
		ORDER BY r.race_end_date ASC
		LIMIT 1`

	var race models.Race
	err := r.db.Raw(query, year, now).First(&race).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &race, err
}