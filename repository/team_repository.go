package repository

import (
	"errors"

	"f1-api-go/internal/models"

	"gorm.io/gorm"
)

type TeamRepository struct {
	db *gorm.DB
}

func NewTeamRepository(db *gorm.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) FindAllTeamsBySeasonName(seasonName string) ([]models.Team, error) {
	query := `
		SELECT DISTINCT t.id, t.name, t.short_name, t.country, t.base, t.created_at, t.updated_at 
		FROM f1_api.team t
		LEFT JOIN f1_api.driver_team_season dts ON t.id = dts.team_id
		LEFT JOIN f1_api.season s ON dts.season_id = s.id
		WHERE s.season_name = ? OR s.season_name IS NULL
		ORDER BY t.id`

	var teams []models.Team
	err := r.db.Raw(query, seasonName).Scan(&teams).Error
	return teams, err
}

func (r *TeamRepository) FindAllByNameContainingIgnoreCase(name string) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.Table("f1_api.team").Where("LOWER(name) LIKE LOWER(?)", "%"+name+"%").Find(&teams).Error
	return teams, err
}

func (r *TeamRepository) FindByName(name string) (*models.Team, error) {
	var team models.Team
	err := r.db.Table("f1_api.team").Where("LOWER(name) LIKE LOWER(?)", "%"+name+"%").First(&team).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &team, err
}

func (r *TeamRepository) FindByShortName(shortName string) (*models.Team, error) {
	var team models.Team
	err := r.db.Table("f1_api.team").Where("short_name = ?", shortName).First(&team).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &team, err
}

func (r *TeamRepository) FindAllByShortName(shortName string) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.Table("f1_api.team").Where("short_name = ?", shortName).Find(&teams).Error
	return teams, err
}
