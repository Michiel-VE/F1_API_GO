package scrapers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"f1-api-go/internal/models"

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
)

type ScrapedDriverService struct {
	driverRepository DriverRepository
	fetcher          *pageFetcher
	historicalLookup *historicalDriverClient
	backfillMode     bool
	seasonYear       string
	logger           *slog.Logger
}

func NewScrapedDriverService(repo DriverRepository, logger *slog.Logger) *ScrapedDriverService {
	return NewScrapedDriverServiceWithOptions(repo, false, "", logger)
}

func NewScrapedDriverServiceWithOptions(
	repo DriverRepository,
	backfillMode bool,
	seasonYear string,
	logger *slog.Logger,
) *ScrapedDriverService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScrapedDriverService{
		driverRepository: repo,
		fetcher:          newPageFetcher(15*time.Second, logger),
		historicalLookup: newHistoricalDriverClient(logger),
		backfillMode:     backfillMode,
		seasonYear:       strings.TrimSpace(seasonYear),
		logger:           logger,
	}
}

func (s *ScrapedDriverService) ScrapeF1Drivers(ctx context.Context) ([]*models.Driver, error) {
	startTime := time.Now()
	var drivers []*models.Driver
	s.logger.Info("Starting scrape of F1 drivers.")

	doc, err := s.fetcher.fetchDocument(ctx, "https://www.formula1.com/en/drivers")
	if err != nil {
		s.logger.Error("Error loading F1 drivers page", slog.String("error", err.Error()))
		return nil, err
	}

	cards := doc.Find(driverCardSelector)
	s.logger.Info("Found driver cards to process.", slog.Int("count", cards.Length()))
	if cards.Length() == 0 {
		return nil, errors.New("no driver cards found on F1.com drivers page - page layout may have changed")
	}

	failed := 0
	cards.Each(func(i int, card *goquery.Selection) {
		href, exists := card.Attr("href")
		if !exists {
			return
		}

		driverPageURL := "https://www.formula1.com" + href
		driverDoc, err := s.fetcher.fetchDocument(ctx, driverPageURL)
		if err != nil {
			failed++
			s.logger.Warn("Could not load driver page.", slog.String("url", driverPageURL), slog.String("error", err.Error()))
			return
		}

		firstName := strings.TrimSpace(driverDoc.Find(driverFirstNameSelector).First().Text())
		lastName := strings.TrimSpace(driverDoc.Find(driverLastNameSelector).First().Text())
		if firstName == "" || lastName == "" {
			failed++
			s.logger.Warn("Skipping driver page with missing name (selectors may be stale).",
				slog.String("url", driverPageURL), slog.String("firstName", firstName), slog.String("lastName", lastName))
			return
		}

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

		var birthday time.Time
		if parsed := s.parseBirthday(strings.TrimSpace(driverDoc.Find(driverBirthdaySelector).First().Text())); parsed != nil {
			birthday = *parsed
		}

		now := time.Now()
		driver := &models.Driver{
			Firstname:       firstName,
			Lastname:        lastName,
			PermanentNumber: permanentNumber,
			DriverCode:      s.getDriverCode(lastName),
			Birthday:        birthday,
			Country:         country,
			CountryCode:     getCountryCode(country),
			Team:            teamName,
		}
		driver.ID = uuid.New()
		driver.CreatedAt = now
		driver.UpdatedAt = now

		drivers = append(drivers, driver)
		s.logger.Debug("Successfully parsed driver", slog.String("name", firstName+" "+lastName))
	})

	if len(drivers) == 0 {
		return nil, fmt.Errorf("no valid drivers parsed from %d cards - selectors may be stale", cards.Length())
	}

	s.logger.Info("Scraping for drivers completed.",
		slog.Int("totalParsed", len(drivers)),
		slog.Int("failed", failed),
		slog.Duration("duration", time.Since(startTime)),
	)

	return drivers, nil
}

func isSameDriverRecord(existing *models.Driver, scraped *models.Driver) bool {
	if existing == nil || scraped == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(existing.Firstname), strings.TrimSpace(scraped.Firstname)) &&
		strings.EqualFold(strings.TrimSpace(existing.Lastname), strings.TrimSpace(scraped.Lastname)) {
		return true
	}
	// If existing only stored a code or empty name (from an incomplete historical row),
	// allow matching by driver code when the name doesn't conflict with another full name.
	if scraped.DriverCode != "" && strings.EqualFold(existing.DriverCode, scraped.DriverCode) {
		if strings.TrimSpace(existing.Firstname) == "" || strings.TrimSpace(existing.Lastname) == "" {
			return true
		}
	}
	return false
}

func applyDriverUpdates(existing *models.Driver, source *models.Driver) bool {
	changed := false
	setString := func(dst *string, value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && *dst != trimmed {
			*dst = trimmed
			changed = true
		}
	}

	setString(&existing.Firstname, source.Firstname)
	setString(&existing.Lastname, source.Lastname)
	setString(&existing.Country, source.Country)
	setString(&existing.CountryCode, source.CountryCode)
	setString(&existing.DriverCode, source.DriverCode)

	if source.Team != "" && strings.TrimSpace(existing.Team) == "" {
		existing.Team = strings.TrimSpace(source.Team)
		changed = true
	} else if source.Team != "" && !strings.EqualFold(strings.TrimSpace(existing.Team), strings.TrimSpace(source.Team)) && existing.Birthday.IsZero() {
		existing.Team = strings.TrimSpace(source.Team)
		changed = true
	}

	if source.PermanentNumber != 0 && existing.PermanentNumber != source.PermanentNumber {
		existing.PermanentNumber = source.PermanentNumber
		changed = true
	}
	if !source.Birthday.IsZero() && !existing.Birthday.Equal(source.Birthday) {
		existing.Birthday = source.Birthday
		changed = true
	}
	return changed
}

func (s *ScrapedDriverService) UpdateDriversFromScraper(ctx context.Context) error {
	scrapedDrivers, err := s.ScrapeF1Drivers(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, scraped := range scrapedDrivers {
		name := scraped.Firstname + " " + scraped.Lastname

		existing, err := s.driverRepository.FindByFirstNameAndLastNameIgnoreCase(ctx, scraped.Firstname, scraped.Lastname)
		if err != nil {
			s.logger.Error("Driver lookup failed", slog.String("name", name), slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("lookup %q: %w", name, err))
			continue
		}
		if existing == nil && len(scraped.DriverCode) == 3 {
			candidate, err := s.driverRepository.FindByDriverCode(ctx, scraped.DriverCode)
			if err != nil {
				s.logger.Error("Driver lookup by code failed", slog.String("code", scraped.DriverCode), slog.String("error", err.Error()))
				errs = append(errs, fmt.Errorf("lookup code %q: %w", scraped.DriverCode, err))
				continue
			}
			if isSameDriverRecord(candidate, scraped) {
				existing = candidate
			}
		}

		if existing == nil {
			if err := s.driverRepository.Save(ctx, scraped); err != nil {
				s.logger.Error("Failed to save new driver", slog.String("name", name), slog.String("error", err.Error()))
				errs = append(errs, fmt.Errorf("save %q: %w", name, err))
				continue
			}
			s.logger.Info("Saved new driver", slog.String("name", name))
			continue
		}

		if !applyDriverUpdates(existing, scraped) {
			s.logger.Debug("Driver data is the same, no update", slog.String("name", name))
			continue
		}

		existing.UpdatedAt = time.Now()
		if err := s.driverRepository.Save(ctx, existing); err != nil {
			s.logger.Error("Failed to update driver", slog.String("name", name), slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("update %q: %w", name, err))
			continue
		}
		s.logger.Info("Updated driver", slog.String("name", name))
	}

	if s.backfillMode && s.historicalLookup != nil {
		if err := s.enrichIncompleteDrivers(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (s *ScrapedDriverService) enrichIncompleteDrivers(ctx context.Context) error {
	allDrivers, err := s.driverRepository.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("loading drivers for backfill enrichment failed: %w", err)
	}

	var errs []error
	for i := range allDrivers {
		d := &allDrivers[i]
		if !d.Birthday.IsZero() && strings.TrimSpace(d.Firstname) != "" && strings.TrimSpace(d.Lastname) != "" {
			continue
		}

		enriched, err := s.historicalLookup.lookup(ctx, s.seasonYear, d.DriverCode, d.Firstname, d.Lastname)
		if err != nil {
			s.logger.Warn("Historical driver lookup failed",
				slog.String("driver_id", d.ID.String()),
				slog.String("code", d.DriverCode),
				slog.String("error", err.Error()))
			continue
		}
		if enriched == nil {
			continue
		}

		if !applyDriverUpdates(d, enriched) {
			continue
		}
		d.UpdatedAt = time.Now()
		if err := s.driverRepository.Save(ctx, d); err != nil {
			errs = append(errs, fmt.Errorf("saving enriched driver %s: %w", d.ID, err))
			continue
		}
		s.logger.Info("Enriched historical driver profile from fallback",
			slog.String("firstname", d.Firstname),
			slog.String("lastname", d.Lastname),
			slog.String("code", d.DriverCode),
			slog.String("birthday", d.Birthday.Format("2006-01-02")),
		)
	}
	return errors.Join(errs...)
}

func (s *ScrapedDriverService) parseBirthday(dateStr string) *time.Time {
	if dateStr == "" {
		return nil
	}
	t, err := time.Parse("02/01/2006", dateStr)
	if err != nil {
		s.logger.Warn("Failed to parse birthday", slog.String("dateStr", dateStr), slog.String("error", err.Error()))
		return nil
	}
	return &t
}

func (s *ScrapedDriverService) getDriverCode(lastname string) string {
	folded := strings.ReplaceAll(normalizeText(lastname), " ", "")
	if len(folded) < 3 {
		return ""
	}
	return strings.ToUpper(folded[:3])
}

func getCountryCode(country string) string {
	if len(country) >= 3 {
		return strings.ToUpper(country[:3])
	}
	return ""
}
