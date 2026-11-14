package repository

import (
	"context"
	"errors"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TeamRepository struct {
	db *gorm.DB
}

func NewTeamRepository(db *gorm.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) table(ctx context.Context) *gorm.DB {
	return DBFrom(ctx, r.db).Table("f1_api.team")
}

// FindAll returns every team. The standings scraper needs all teams: the old
// "teams of this season" query hid any team without a points row yet, so those
// drivers failed with "team not found" at the start of a season.
func (r *TeamRepository) FindAll(ctx context.Context) ([]models.Team, error) {
	var teams []models.Team
	err := r.table(ctx).Order("created_at ASC").Find(&teams).Error
	return teams, err
}

func (r *TeamRepository) FindAllTeamsBySeasonName(ctx context.Context, seasonName string) ([]models.Team, error) {
	query := `
		SELECT DISTINCT t.id, t.name, t.short_name, t.country, t.base, t.created_at, t.updated_at 
		FROM f1_api.team t
		LEFT JOIN f1_api.driver_team_season dts ON t.id = dts.team_id
		LEFT JOIN f1_api.season s ON dts.season_id = s.id
		WHERE s.season_name = ? OR s.season_name IS NULL
		ORDER BY t.id`

	var teams []models.Team
	err := DBFrom(ctx, r.db).Raw(query, seasonName).Scan(&teams).Error
	return teams, err
}

func (r *TeamRepository) FindAllByNameContainingIgnoreCase(ctx context.Context, name string) ([]models.Team, error) {
	var teams []models.Team
	err := r.table(ctx).Where("LOWER(name) LIKE LOWER(?)", "%"+name+"%").Find(&teams).Error
	return teams, err
}

func (r *TeamRepository) FindByName(ctx context.Context, name string) (*models.Team, error) {
	var team models.Team
	err := r.table(ctx).Where("LOWER(name) LIKE LOWER(?)", "%"+name+"%").First(&team).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &team, nil
}

func (r *TeamRepository) FindByShortName(ctx context.Context, shortName string) (*models.Team, error) {
	var team models.Team
	err := r.table(ctx).Where("short_name = ?", shortName).First(&team).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &team, nil
}

// FindAllByShortName is ordered oldest-first so that when duplicates exist the
// team that most likely holds the driver_team_season references comes first
// and is the one that is kept.
func (r *TeamRepository) FindAllByShortName(ctx context.Context, shortName string) ([]models.Team, error) {
	var teams []models.Team
	err := r.table(ctx).Where("short_name = ?", shortName).Order("created_at ASC, id ASC").Find(&teams).Error
	return teams, err
}

func (r *TeamRepository) Save(ctx context.Context, team *models.Team) error {
	return r.table(ctx).Save(team).Error
}

func (r *TeamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.table(ctx).Where("id = ?", id).Delete(&models.Team{}).Error
}
