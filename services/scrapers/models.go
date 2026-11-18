package scrapers

import (
	"context"
	"errors"
	"time"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
)

// ErrSeasonNotFound lets callers tell "this season isn't in the database"
// (often fine, e.g. last year's season in January) from a real failure.
var ErrSeasonNotFound = errors.New("season not found")

type Race struct {
	ID                uuid.UUID
	Name              string
	Country           string
	StartDate         time.Time
	EndDate           time.Time
	RaceStartDate     time.Time
	RaceEndDate       time.Time
	IsSprint          bool
	Status            *RaceStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ExternalResultURL *string
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
	SprintPoints  float64
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
	FindAllFinishedRacesMissingResults(ctx context.Context, now time.Time, seasonName string) ([]models.Race, error)
	FindFirstFinishedRaceMissingResults(ctx context.Context, now time.Time, seasonName string) (*models.Race, error)
	FindCompletedNonSprintRaces(ctx context.Context, now time.Time, seasonName string) ([]models.Race, error)
	FindByNameAndRaceStartDateAndRaceEndDate(ctx context.Context, name string, start, end time.Time) (*Race, error)
	FindByRaceStartDateInSeason(ctx context.Context, seasonName string, start time.Time) (*Race, error)
	UpdateDetails(ctx context.Context, raceID uuid.UUID, name, country string, start, end time.Time) error
	UpdatePastScheduledRacesToCompleted(ctx context.Context, now time.Time, seasonName string) error
	Save(ctx context.Context, race *Race) (*Race, error)
}

type SeasonRepository interface {
	FindBySeasonName(ctx context.Context, seasonName string) (*Season, error)
	Save(ctx context.Context, season *Season) (*Season, error)
}

// RaceDriverRepository is what the race-result scraper needs.
type RaceDriverRepository interface {
	FindByDriverCode(ctx context.Context, driverCode string) (*models.Driver, error)
}

// DriverRepository is what the driver scraper needs.
type DriverRepository interface {
	FindByFirstNameAndLastNameIgnoreCase(ctx context.Context, firstname, lastname string) (*models.Driver, error)
	FindByDriverCode(ctx context.Context, driverCode string) (*models.Driver, error)
	Save(ctx context.Context, driver *models.Driver) error
}

// DriverStandingRepository is what the standings scraper needs. Lookup order is
// driver code, then first+last name, then a surname that is unique in the table.
type DriverStandingRepository interface {
	FindByDriverCode(ctx context.Context, driverCode string) (*models.Driver, error)
	FindByFirstNameAndLastNameIgnoreCase(ctx context.Context, firstname, lastname string) (*models.Driver, error)
	FindAllByLastNameIgnoreCase(ctx context.Context, lastname string) ([]*models.Driver, error)
}
