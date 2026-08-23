package repository

import (
	"context"
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

func (r *DriverRepository) Save(ctx context.Context, driver *models.Driver) error {
	return r.db.WithContext(ctx).Table("f1_api.driver").Save(driver).Error
}

func (r *DriverRepository) FindAllBySeasonName(season string) ([]models.Driver, error) {
	var drivers []models.Driver
	err := r.db.Table("f1_api.driver").
		Select("f1_api.driver.*, dts.points").
		Joins("JOIN f1_api.driver_team_season dts ON dts.driver_id = f1_api.driver.id").
		Joins("JOIN f1_api.season s ON s.id = dts.season_id").
		Preload("DriverTeamSeasons", func(db *gorm.DB) *gorm.DB {
			return db.Joins("JOIN f1_api.season s ON s.id = driver_team_season.season_id").
				Where("s.season_name = ?", season)
		}).
		Preload("DriverTeamSeasons.Team").
		Preload("DriverTeamSeasons.Season").
		Where("s.season_name = ?", season).
		Order("dts.points DESC").
		Find(&drivers).Error

	return drivers, err
}

func (r *DriverRepository) FindByLastNameIgnoreCase(lastName string) (*models.Driver, error) {
	var driver models.Driver
	err := r.db.Table("f1_api.driver").Where("LOWER(lastname) = LOWER(?)", lastName).First(&driver).Error
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

func (r *DriverRepository) FindCareerHistoryByLastNameIgnoreCase(lastName string) ([]models.Driver, error) {
	var drivers []models.Driver
	err := r.db.Table("f1_api.driver").
		Preload("DriverTeamSeasons", func(db *gorm.DB) *gorm.DB {
			return db.Order("driver_team_season.points DESC")
		}).
		Preload("DriverTeamSeasons.Team").
		Preload("DriverTeamSeasons.Season").
		Where("LOWER(lastname) LIKE LOWER(?)", "%"+lastName+"%").
		Order("f1_api.driver.id").
		Find(&drivers).Error

	return drivers, err
}

func (r *DriverRepository) FindByFirstNameIgnoreCase(firstName string) (*models.Driver, error) {
	var driver models.Driver
	err := r.db.Table("f1_api.driver").Where("LOWER(firstname) = LOWER(?)", firstName).First(&driver).Error
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

func (r *DriverRepository) FindByID(id uuid.UUID) (*models.Driver, error) {
	var driver models.Driver
	err := r.db.Table("f1_api.driver").Where("id = ?", id).First(&driver).Error
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

func (r *DriverRepository) FindByFirstNameAndLastNameIgnoreCase(firstName, lastName string) (*models.Driver, error) {
	var driver models.Driver
	err := r.db.Table("f1_api.driver").Where("LOWER(firstname) = LOWER(?) AND LOWER(lastname) = LOWER(?)", firstName, lastName).First(&driver).Error
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

func (r *DriverRepository) FindByPermanentNumber(permanentNumber int) ([]models.Driver, error) {
	var drivers []models.Driver
	err := r.db.Table("f1_api.driver").Where("permanent_number = ?", permanentNumber).Find(&drivers).Error
	return drivers, err
}

func (r *DriverRepository) FindByDriverCode(driverCode string) (*models.Driver, error) {
	var driver models.Driver
	err := r.db.Table("f1_api.driver").Where("driver_code = ?", driverCode).First(&driver).Error
	if err != nil {
		return nil, err
	}
	return &driver, nil
}

func (r *DriverRepository) FindAll() ([]models.Driver, error) {
	var drivers []models.Driver
	err := r.db.Table("f1_api.driver").Find(&drivers).Error
	return drivers, err
}
