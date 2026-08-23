package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
)

const (
	driverCardSelector      = `a[data-f1rd-a7s-click="driver_card_click"]`
	driverFirstNameSelector = `h1 > span.-mb-px-8 > span`
	driverLastNameSelector  = `h1 > span.typography-module_display-3-xl-black__CWhFe`
	driverBirthdaySelector  = `dt:contains(Date of Birth) + dd`
	driverInfoGroupSelector = `.grow.flex.flex-col.justify-end.md\:justify-around.gap-px-16.text-static-static-1 > div:nth-child(1) > div.flex.gap-px-12.items-center`
	driverCountrySelector   = `p:nth-child(2)`
	driverTeamSelector      = `p:nth-child(3)`
	driverNumberSelector    = `p:nth-child(5)`
	driverUserAgent         = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36"
)

type Driver struct {
	ID              uuid.UUID
	Firstname       string
	Lastname        string
	PermanentNumber int
	DriverCode      string
	Birthday        *time.Time
	Country         string
	CountryCode     string
	Team            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DriverRepository interface {
	FindByFirstnameAndLastnameIgnoreCase(ctx context.Context, firstname, lastname string) (*Driver, error)
	Save(ctx context.Context, driver *Driver) error
}

type ScrapedDriverService struct {
	driverRepository DriverRepository
	httpClient       *http.Client
	logger           *slog.Logger
}

func NewScrapedDriverService(repo DriverRepository, logger *slog.Logger) *ScrapedDriverService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScrapedDriverService{
		driverRepository: repo,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

func (s *ScrapedDriverService) fetchDocument(ctx context.Context, url string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", driverUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http request failed with status: %d", resp.StatusCode)
	}

	return goquery.NewDocumentFromReader(resp.Body)
}

func (s *ScrapedDriverService) ScrapeF1Drivers(ctx context.Context) ([]*Driver, error) {
	startTime := time.Now()
	var drivers []*Driver
	s.logger.Info("Starting scrape of F1 drivers.")

	doc, err := s.fetchDocument(ctx, "https://www.formula1.com/en/drivers")
	if err != nil {
		s.logger.Error("Error loading F1 drivers page", slog.String("error", err.Error()))
		return nil, err
	}

	cards := doc.Find(driverCardSelector)
	s.logger.Info("Found driver cards to process.", slog.Int("count", cards.Length()))

	cards.Each(func(i int, card *goquery.Selection) {
		href, exists := card.Attr("href")
		if !exists {
			return
		}

		driverPageURL := "https://www.formula1.com" + href
		driverDoc, err := s.fetchDocument(ctx, driverPageURL)
		if err != nil {
			s.logger.Warn("Could not parse driver card.", slog.String("url", driverPageURL), slog.String("error", err.Error()))
			return
		}

		firstName := strings.TrimSpace(driverDoc.Find(driverFirstNameSelector).First().Text())
		lastName := strings.TrimSpace(driverDoc.Find(driverLastNameSelector).First().Text())

		infoGroup := driverDoc.Find(driverInfoGroupSelector).First()
		var country, teamName string
		var permanentNumber int

		if infoGroup.Length() > 0 {
			country = strings.TrimSpace(infoGroup.Find(driverCountrySelector).First().Text())
			teamName = strings.TrimSpace(infoGroup.Find(driverTeamSelector).First().Text())
			numberText := strings.TrimSpace(infoGroup.Find(driverNumberSelector).First().Text())

			if numberText != "" {
				num, err := strconv.Atoi(numberText)
				if err != nil {
					s.logger.Warn("Could not parse permanent number", slog.String("numberText", numberText))
				} else {
					permanentNumber = num
				}
			}
		}

		countryCode := getCountryCode(country)
		birthday := s.parseBirthday(strings.TrimSpace(driverDoc.Find(driverBirthdaySelector).First().Text()))
		driverCode := s.getDriverCode(lastName)
		now := time.Now()

		driver := &Driver{
			ID:              uuid.New(),
			Firstname:       firstName,
			Lastname:        lastName,
			PermanentNumber: permanentNumber,
			DriverCode:      driverCode,
			Birthday:        birthday,
			Country:         country,
			CountryCode:     countryCode,
			Team:            teamName,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		drivers = append(drivers, driver)
		s.logger.Debug("Successfully parsed driver", slog.String("name", firstName+" "+lastName))
	})

	s.logger.Info("Scraping for drivers completed.",
		slog.Int("totalParsed", len(drivers)),
		slog.Duration("duration", time.Since(startTime)),
	)

	return drivers, nil
}

func (s *ScrapedDriverService) UpdateDriversFromScraper(ctx context.Context) error {
	scrapedDrivers, err := s.ScrapeF1Drivers(ctx)
	if err != nil {
		return err
	}

	for _, scrapedDriver := range scrapedDrivers {
		existing, err := s.driverRepository.FindByFirstnameAndLastnameIgnoreCase(ctx, scrapedDriver.Firstname, scrapedDriver.Lastname)

		if err != nil || existing == nil {
			if err := s.driverRepository.Save(ctx, scrapedDriver); err != nil {
				s.logger.Error("Failed to save new driver", slog.String("lastname", scrapedDriver.Lastname), slog.String("error", err.Error()))
				continue
			}
			s.logger.Info("Saved new driver", slog.String("lastname", scrapedDriver.Lastname))
		} else {
			isDifferent := false

			if existing.Firstname != scrapedDriver.Firstname {
				existing.Firstname = scrapedDriver.Firstname
				isDifferent = true
			}
			if existing.Lastname != scrapedDriver.Lastname {
				existing.Lastname = scrapedDriver.Lastname
				isDifferent = true
			}
			if (existing.Birthday == nil && scrapedDriver.Birthday != nil) ||
				(existing.Birthday != nil && scrapedDriver.Birthday == nil) ||
				(existing.Birthday != nil && scrapedDriver.Birthday != nil && !existing.Birthday.Equal(*scrapedDriver.Birthday)) {
				existing.Birthday = scrapedDriver.Birthday
				isDifferent = true
			}
			if existing.Country != scrapedDriver.Country {
				existing.Country = scrapedDriver.Country
				isDifferent = true
			}
			if existing.Team != scrapedDriver.Team {
				existing.Team = scrapedDriver.Team
				isDifferent = true
			}

			if isDifferent {
				existing.UpdatedAt = scrapedDriver.UpdatedAt
				if err := s.driverRepository.Save(ctx, existing); err != nil {
					s.logger.Error("Failed to update driver", slog.String("lastname", existing.Lastname), slog.String("error", err.Error()))
					continue
				}
				s.logger.Info("Updated driver", slog.String("name", existing.Firstname+" "+existing.Lastname))
			} else {
				s.logger.Info("Driver data is the same, no update", slog.String("name", scrapedDriver.Firstname+" "+scrapedDriver.Lastname))
			}
		}
	}
	return nil
}

func (s *ScrapedDriverService) parseBirthday(dateStr string) *time.Time {
	t, err := time.Parse("02/01/2006", dateStr)
	if err != nil {
		s.logger.Error("Failed to parse birthday", slog.String("dateStr", dateStr), slog.String("error", err.Error()))
		return nil
	}
	return &t
}

func (s *ScrapedDriverService) getDriverCode(lastname string) string {
	if len(lastname) < 3 {
		return ""
	}
	return strings.ToUpper(lastname[:3])
}

// Stub implementation for compilation
func getCountryCode(country string) string {
	if len(country) >= 3 {
		return strings.ToUpper(country[:3])
	}
	return ""
}