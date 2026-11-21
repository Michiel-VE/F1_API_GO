package repository

import (
	"context"
	"errors"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DriverRepository struct {
	db *gorm.DB
}

func NewDriverRepository(db *gorm.DB) *DriverRepository {
	return &DriverRepository{db: db}
}

func (r *DriverRepository) table(ctx context.Context) *gorm.DB {
	return DBFrom(ctx, r.db).Table("f1_api.driver")
}

// findOne returns (nil, nil) when nothing matches and an error only for real
// database failures.
func (r *DriverRepository) findOne(ctx context.Context, query string, args ...any) (*models.Driver, error) {
	var driver models.Driver
	err := r.table(ctx).Where(query, args...).First(&driver).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

func (r *DriverRepository) Save(ctx context.Context, driver *models.Driver) error {
	return r.table(ctx).Save(driver).Error
}

// FindAllBySeasonName returns each driver once (a driver with two team rows in
// the same season used to appear twice), ordered by their season points. The
// season total lives on the newest row, so MAX is used rather than SUM.
func (r *DriverRepository) FindAllBySeasonName(ctx context.Context, season string) ([]models.Driver, error) {
	var drivers []models.Driver
	err := r.table(ctx).
		Select("f1_api.driver.*").
		Joins(`JOIN (
			SELECT dts.driver_id, MAX(dts.points) AS season_points
			FROM f1_api.driver_team_season dts
			JOIN f1_api.season s ON s.id = dts.season_id
			WHERE s.season_name = ?
			GROUP BY dts.driver_id
		) pts ON pts.driver_id = f1_api.driver.id`, season).
		Preload("DriverTeamSeasons", func(db *gorm.DB) *gorm.DB {
			return db.Joins("JOIN f1_api.season s ON s.id = driver_team_season.season_id").
				Where("s.season_name = ?", season)
		}).
		Preload("DriverTeamSeasons.Team").
		Preload("DriverTeamSeasons.Season").
		Order("pts.season_points DESC").
		Find(&drivers).Error

	return drivers, err
}

func (r *DriverRepository) FindByLastNameIgnoreCase(ctx context.Context, lastName string) (*models.Driver, error) {
	return r.findOne(ctx, "LOWER(lastname) = LOWER(?)", lastName)
}

// FindAllByLastNameIgnoreCase is used where a surname alone may be ambiguous
// (historical drivers share surnames) so callers can detect that.
func (r *DriverRepository) FindAllByLastNameIgnoreCase(ctx context.Context, lastName string) ([]models.Driver, error) {
	var drivers []models.Driver
	err := r.table(ctx).Where("LOWER(lastname) = LOWER(?)", lastName).Find(&drivers).Error
	return drivers, err
}

func (r *DriverRepository) FindCareerHistoryByLastNameIgnoreCase(ctx context.Context, lastName string) ([]models.Driver, error) {
	var drivers []models.Driver
	err := r.table(ctx).
		Preload("DriverTeamSeasons", func(db *gorm.DB) *gorm.DB {
			return db.Order("driver_team_season.points DESC")
		}).
		Preload("DriverTeamSeasons.Team").
		Preload("DriverTeamSeasons.Season").
		Where("LOWER(lastname) LIKE LOWER(?)", "%"+escapeLike(lastName)+"%").
		Order("f1_api.driver.id").
		Find(&drivers).Error

	return drivers, err
}

func (r *DriverRepository) SearchByNameIgnoreCase(ctx context.Context, query string) ([]models.Driver, error) {
	pattern := "%" + escapeLike(query) + "%"
	var drivers []models.Driver
	err := r.table(ctx).
		Where(
			"LOWER(firstname) LIKE LOWER(?) OR LOWER(lastname) LIKE LOWER(?) OR LOWER(TRIM(firstname || ' ' || lastname)) LIKE LOWER(?)",
			pattern,
			pattern,
			pattern,
		).
		Order("lastname ASC, firstname ASC").
		Find(&drivers).Error
	return drivers, err
}

func (r *DriverRepository) FindByFirstNameIgnoreCase(ctx context.Context, firstName string) (*models.Driver, error) {
	return r.findOne(ctx, "LOWER(firstname) = LOWER(?)", firstName)
}

// FindByID also preloads the driver's team seasons, since the endpoint that
// uses it returns a "driver with seasons" response.
func (r *DriverRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Driver, error) {
	var driver models.Driver
	err := r.table(ctx).
		Preload("DriverTeamSeasons").
		Preload("DriverTeamSeasons.Team").
		Preload("DriverTeamSeasons.Season").
		Where("id = ?", id).
		First(&driver).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

func (r *DriverRepository) FindByFirstNameAndLastNameIgnoreCase(ctx context.Context, firstName, lastName string) (*models.Driver, error) {
	return r.findOne(ctx, "LOWER(firstname) = LOWER(?) AND LOWER(lastname) = LOWER(?)", firstName, lastName)
}

func (r *DriverRepository) FindByPermanentNumber(ctx context.Context, permanentNumber int) ([]models.Driver, error) {
	var drivers []models.Driver
	err := r.table(ctx).Where("permanent_number = ?", permanentNumber).Find(&drivers).Error
	return drivers, err
}

func (r *DriverRepository) FindByDriverCode(ctx context.Context, driverCode string) (*models.Driver, error) {
	return r.findOne(ctx, "driver_code = ?", driverCode)
}

func (r *DriverRepository) FindAll(ctx context.Context) ([]models.Driver, error) {
	var drivers []models.Driver
	err := r.table(ctx).Find(&drivers).Error
	return drivers, err
}
