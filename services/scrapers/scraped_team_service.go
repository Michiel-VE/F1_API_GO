package scrapers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
)

const (
	teamCardSelector    = `a[data-f1rd-a7s-click="team_card_click"]`
	teamCardShortname   = "div.relative.z-40.w-full.flex.justify-center.gap-px-32 > h1"
	teamCardName        = "dl.DataGrid-module_dataGrid__Zk5Y8.DataGrid-module_columns__rcgN8 > div > dd"
	teamCardBaseCountry = "dl.DataGrid-module_dataGrid__Zk5Y8.DataGrid-module_columns__rcgN8 > div:nth-child(2) > dd"
)

type ScrapedTeamService struct {
	teamRepository TeamRepository
	fetcher        *pageFetcher
	logger         *slog.Logger
}

func NewScrapedTeamService(repo TeamRepository, logger *slog.Logger) *ScrapedTeamService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScrapedTeamService{
		teamRepository: repo,
		fetcher:        newPageFetcher(15*time.Second, logger),
		logger:         logger,
	}
}

func (s *ScrapedTeamService) ScrapeF1Teams(ctx context.Context) ([]*Team, error) {
	startTime := time.Now()
	var teams []*Team
	s.logger.Info("Starting scrape of F1 teams.")

	doc, err := s.fetcher.fetchDocument(ctx, "https://www.formula1.com/en/teams")
	if err != nil {
		s.logger.Error("Error loading F1 teams page", slog.String("error", err.Error()))
		return nil, err
	}

	cards := doc.Find(teamCardSelector)
	s.logger.Info("Found team cards to process.", slog.Int("count", cards.Length()))
	if cards.Length() == 0 {
		return nil, errors.New("no team cards found on F1.com teams page - page layout may have changed")
	}

	failed := 0
	cards.Each(func(i int, card *goquery.Selection) {
		href, exists := card.Attr("href")
		if !exists {
			return
		}

		teamPageURL := "https://www.formula1.com" + href
		s.logger.Debug("Processing team page", slog.String("url", teamPageURL))

		teamDoc, err := s.fetcher.fetchDocument(ctx, teamPageURL)
		if err != nil {
			failed++
			s.logger.Warn("Could not load team page.", slog.String("url", teamPageURL), slog.String("error", err.Error()))
			return
		}

		shortName := strings.TrimSpace(teamDoc.Find(teamCardShortname).First().Text())
		name := strings.TrimSpace(teamDoc.Find(teamCardName).First().Text())
		townCountry := strings.TrimSpace(teamDoc.Find(teamCardBaseCountry).First().Text())

		// Never save a team with an empty short name: an empty short name later
		// matches every scraped team string in the standings scraper.
		if shortName == "" || name == "" {
			failed++
			s.logger.Warn("Skipping team page with missing name/short name (selectors may be stale).",
				slog.String("url", teamPageURL), slog.String("name", name), slog.String("shortName", shortName))
			return
		}

		var country, town string
		parts := strings.Split(townCountry, ",")
		if len(parts) >= 2 {
			town = strings.TrimSpace(parts[0])
			country = strings.TrimSpace(parts[1])
		} else if len(parts) == 1 {
			town = strings.TrimSpace(parts[0])
		}

		now := time.Now()
		teams = append(teams, &Team{
			ID:        uuid.New(),
			Name:      name,
			ShortName: shortName,
			Country:   country,
			Base:      town,
			CreatedAt: now,
			UpdatedAt: now,
		})
	})

	if len(teams) == 0 {
		return nil, fmt.Errorf("no valid teams parsed from %d cards - selectors may be stale", cards.Length())
	}

	s.logger.Info("Scraping for teams completed.",
		slog.Int("totalParsed", len(teams)),
		slog.Int("failed", failed),
		slog.Duration("duration", time.Since(startTime)),
	)

	return teams, nil
}

func (s *ScrapedTeamService) UpdateTeamsFromScraper(ctx context.Context) error {
	scrapedTeams, err := s.ScrapeF1Teams(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, scrapedTeam := range scrapedTeams {
		existingTeams, err := s.teamRepository.FindAllByShortName(ctx, scrapedTeam.ShortName)
		if err != nil {
			s.logger.Error("Failed to query existing teams", slog.String("shortName", scrapedTeam.ShortName), slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("query %q: %w", scrapedTeam.ShortName, err))
			continue
		}

		if len(existingTeams) == 0 {
			if err := s.teamRepository.Save(ctx, scrapedTeam); err != nil {
				s.logger.Error("Failed to save new team", slog.String("name", scrapedTeam.Name), slog.String("error", err.Error()))
				errs = append(errs, fmt.Errorf("save %q: %w", scrapedTeam.Name, err))
				continue
			}
			s.logger.Info("Saved new team", slog.String("name", scrapedTeam.Name))
			continue
		}

		// The repository returns oldest-first, so the kept team is the one most
		// likely to hold the driver_team_season references.
		teamToUpdate := existingTeams[0]

		isDifferent := teamToUpdate.Name != scrapedTeam.Name ||
			teamToUpdate.Base != scrapedTeam.Base ||
			teamToUpdate.Country != scrapedTeam.Country

		if isDifferent {
			teamToUpdate.Name = scrapedTeam.Name
			teamToUpdate.Base = scrapedTeam.Base
			teamToUpdate.Country = scrapedTeam.Country
			teamToUpdate.UpdatedAt = time.Now()

			if err := s.teamRepository.Save(ctx, teamToUpdate); err != nil {
				s.logger.Error("Failed to update team", slog.String("shortName", teamToUpdate.ShortName), slog.String("error", err.Error()))
				errs = append(errs, fmt.Errorf("update %q: %w", teamToUpdate.ShortName, err))
				continue
			}
			s.logger.Info("Updated existing team", slog.String("shortName", teamToUpdate.ShortName))
		}

		if len(existingTeams) > 1 {
			for i := 1; i < len(existingTeams); i++ {
				if err := s.teamRepository.Delete(ctx, existingTeams[i]); err != nil {
					s.logger.Error("Failed to delete duplicate team", slog.String("error", err.Error()))
				}
			}
			s.logger.Info("Cleaned up duplicates for team",
				slog.Int("count", len(existingTeams)-1),
				slog.String("shortName", scrapedTeam.ShortName),
			)
		}
	}
	return errors.Join(errs...)
}
