package repository

import (
	"context"
	"errors"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TeamPredictionRepository struct {
	db *gorm.DB
}

func NewTeamPredictionRepository(db *gorm.DB) *TeamPredictionRepository {
	return &TeamPredictionRepository{db: db}
}

// The predicted teams live in f1_api.team_prediction_teams
// (prediction_id, team_id, prediction_order). prediction_order is the rank the
// user gave the team. gorm's many2many support knows nothing about that column:
// it never wrote it, and reading back returned the teams in the teams table's
// order, so the user's ranking was lost. The teams are therefore written and
// read by hand below.

type predictionTeamLink struct {
	PredictionID uuid.UUID
	TeamID       uuid.UUID
}

// loadPredictedTeams fills PredictedTeams (in the user's order) for every prediction.
func loadPredictedTeams(db *gorm.DB, predictions []*models.TeamPrediction) error {
	if len(predictions) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(predictions))
	byID := make(map[uuid.UUID]*models.TeamPrediction, len(predictions))
	for _, p := range predictions {
		if p == nil {
			continue
		}
		p.PredictedTeams = nil
		ids = append(ids, p.ID)
		byID[p.ID] = p
	}
	if len(ids) == 0 {
		return nil
	}

	var links []predictionTeamLink
	err := db.Raw(`
		SELECT prediction_id, team_id
		FROM f1_api.team_prediction_teams
		WHERE prediction_id IN ?
		ORDER BY prediction_id, prediction_order ASC NULLS LAST, team_id`, ids).Scan(&links).Error
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}

	teamIDs := make([]uuid.UUID, 0, len(links))
	seen := make(map[uuid.UUID]struct{}, len(links))
	for _, l := range links {
		if _, ok := seen[l.TeamID]; !ok {
			seen[l.TeamID] = struct{}{}
			teamIDs = append(teamIDs, l.TeamID)
		}
	}

	var teams []models.Team
	if err := db.Table("f1_api.team").Where("id IN ?", teamIDs).Find(&teams).Error; err != nil {
		return err
	}
	teamByID := make(map[uuid.UUID]models.Team, len(teams))
	for _, t := range teams {
		teamByID[t.ID] = t
	}

	for _, l := range links {
		p, okP := byID[l.PredictionID]
		t, okT := teamByID[l.TeamID]
		if okP && okT {
			p.PredictedTeams = append(p.PredictedTeams, t)
		}
	}
	return nil
}

// FindPersonalByUserId returns the user's personal prediction (the one that is
// not attached to a pool), or (nil, nil) when there is none.
func (r *TeamPredictionRepository) FindPersonalByUserId(ctx context.Context, userID uuid.UUID) (*models.TeamPrediction, error) {
	db := DBFrom(ctx, r.db)

	var prediction models.TeamPrediction
	err := db.Where("user_id = ? AND pool_id IS NULL", userID).First(&prediction).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := loadPredictedTeams(db, []*models.TeamPrediction{&prediction}); err != nil {
		return nil, err
	}
	return &prediction, nil
}

func (r *TeamPredictionRepository) FindByUserIdAndPoolId(ctx context.Context, userID, poolID uuid.UUID) (*models.TeamPrediction, error) {
	db := DBFrom(ctx, r.db)

	var prediction models.TeamPrediction
	err := db.Where("user_id = ? AND pool_id = ?", userID, poolID).First(&prediction).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := loadPredictedTeams(db, []*models.TeamPrediction{&prediction}); err != nil {
		return nil, err
	}
	return &prediction, nil
}

func (r *TeamPredictionRepository) FindByPoolId(ctx context.Context, poolID uuid.UUID) ([]*models.TeamPrediction, error) {
	db := DBFrom(ctx, r.db)

	var predictions []*models.TeamPrediction
	if err := db.Preload("User").Where("pool_id = ?", poolID).Find(&predictions).Error; err != nil {
		return nil, err
	}
	if err := loadPredictedTeams(db, predictions); err != nil {
		return nil, err
	}
	return predictions, nil
}

// Create inserts a prediction and its ranked teams. The team links are written
// with their position (0 = the user's first pick). When the database rejects the
// insert because the user already has such a prediction (see
// db/prediction_constraints.sql) it returns ErrDuplicate.
func (r *TeamPredictionRepository) Create(ctx context.Context, prediction *models.TeamPrediction) error {
	db := DBFrom(ctx, r.db)

	err := db.Omit("PredictedTeams").Create(prediction).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicate
	}
	if err != nil {
		return err
	}

	for position, team := range prediction.PredictedTeams {
		err := db.Exec(`
			INSERT INTO f1_api.team_prediction_teams (prediction_id, team_id, prediction_order)
			VALUES (?::uuid, ?::uuid, ?)`, prediction.ID, team.ID, position).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a prediction and its team links (the teams themselves are kept).
func (r *TeamPredictionRepository) Delete(ctx context.Context, prediction *models.TeamPrediction) error {
	db := DBFrom(ctx, r.db)
	if err := db.Exec(`DELETE FROM f1_api.team_prediction_teams WHERE prediction_id = ?::uuid`, prediction.ID).Error; err != nil {
		return err
	}
	return db.Delete(prediction).Error
}

// DeleteByPoolId removes every prediction of a pool (used when a pool is dissolved).
func (r *TeamPredictionRepository) DeleteByPoolId(ctx context.Context, poolID uuid.UUID) error {
	db := DBFrom(ctx, r.db)

	err := db.Exec(`
		DELETE FROM f1_api.team_prediction_teams
		WHERE prediction_id IN (SELECT id FROM f1_api.team_predictions WHERE pool_id = ?::uuid)`, poolID).Error
	if err != nil {
		return err
	}
	return db.Exec(`DELETE FROM f1_api.team_predictions WHERE pool_id = ?::uuid`, poolID).Error
}
