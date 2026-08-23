package scrapers

import (
	"context"
	"time"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
)

type Race struct {
	ID            uuid.UUID
	Name          string
	Country       string
	StartDate     time.Time
	EndDate       time.Time
	RaceStartDate time.Time
	RaceEndDate   time.Time
	Status        *RaceStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Season struct {
	ID         uuid.UUID
	SeasonName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Team struct {
	ID        uuid.UUID
	Name      string
	ShortName string
	Country   string
	Base      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ScrapedStanding struct {
	Position    int
	FirstName   string
	LastName    string
	DriverCode  string
	Nationality string
	Team        string
	Points      int
}

type DriverRaceResult struct {
	ID            uuid.UUID
	Driver        *models.Driver
	Race          *Race
	Season        *Season
	Points        float64
	LapsCompleted int
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DriverTeamSeason struct {
	ID        uuid.UUID
	Driver    *models.Driver
	Season    *Season
	Team      *Team
	Points    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TransactionManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type TeamRepository interface {
	FindAll(ctx context.Context) ([]*Team, error)
	FindAllByShortName(ctx context.Context, shortName string) ([]*Team, error)
	Save(ctx context.Context, team *Team) error
	Delete(ctx context.Context, team *Team) error
}

type DriverTeamSeasonRepository interface {
	FindByDriverAndSeasonAndTeam(ctx context.Context, driver *models.Driver, season *Season, team *Team) (*DriverTeamSeason, error)
	Save(ctx context.Context, dts *DriverTeamSeason) (*DriverTeamSeason, error)
}

type DriverRaceResultRepository interface {
	ExistsByDriverAndRaceAndSeason(ctx context.Context, driver *models.Driver, race *Race, season *Season) (bool, error)
	Save(ctx context.Context, result *DriverRaceResult) error
}

type RaceRepository interface {
	FindFirstFinishedRaceMissingResults(ctx context.Context, now time.Time, seasonName string) (*Race, error)
	FindByNameAndRaceStartDateAndRaceEndDate(ctx context.Context, name string, start, end time.Time) (*Race, error)
	Save(ctx context.Context, race *Race) (*Race, error)
}

type SeasonRepository interface {
	FindBySeasonName(ctx context.Context, seasonName string) (*Season, error)
	Save(ctx context.Context, season *Season) (*Season, error)
}

type RaceDriverRepository interface {
	FindByFirstNameAndLastNameIgnoreCase(firstname, lastname string) (*models.Driver, error)
	FindByDriverCode(ctx context.Context, driverCode string) (*models.Driver, error)
	Save(ctx context.Context, driver *models.Driver) error
}

type DriverRepository interface {
	FindByFirstNameAndLastNameIgnoreCase(firstname, lastname string) (*models.Driver, error)
	Save(ctx context.Context, driver *models.Driver) error
}

type DriverStandingRepository interface {
	FindByLastNameIgnoreCase(lastname string) (*models.Driver, error)
	FindByFirstNameIgnoreCase(firstname string) (*models.Driver, error)
}
