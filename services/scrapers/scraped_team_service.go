package scrapers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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
	teamUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36"
)

type ScrapedTeamService struct {
	teamRepository TeamRepository
	httpClient     *http.Client
	logger         *slog.Logger
}

func NewScrapedTeamService(repo TeamRepository, logger *slog.Logger) *ScrapedTeamService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScrapedTeamService{
		teamRepository: repo,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

func (s *ScrapedTeamService) fetchDocument(ctx context.Context, url string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", teamUserAgent)

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

func (s *ScrapedTeamService) ScrapeF1Teams(ctx context.Context) ([]*Team, error) {
	startTime := time.Now()
	var teams []*Team
	s.logger.Info("Starting scrape of F1 teams.")

	doc, err := s.fetchDocument(ctx, "https://www.formula1.com/en/teams")
	if err != nil {
		s.logger.Error("Error loading F1 teams page", slog.String("error", err.Error()))
		return nil, err
	}

	cards := doc.Find(teamCardSelector)
	s.logger.Info("Found team cards to process.", slog.Int("count", cards.Length()))

	cards.Each(func(i int, card *goquery.Selection) {
		href, exists := card.Attr("href")
		if !exists {
			return
		}

		teamPageURL := "https://www.formula1.com" + href
		s.logger.Debug("Processing team page", slog.String("url", teamPageURL))

		teamDoc, err := s.fetchDocument(ctx, teamPageURL)
		if err != nil {
			s.logger.Warn("Could not parse team card.", slog.String("error", err.Error()))
			return
		}

		shortName := strings.TrimSpace(teamDoc.Find(teamCardShortname).First().Text())
		name := strings.TrimSpace(teamDoc.Find(teamCardName).First().Text())
		townCountry := strings.TrimSpace(teamDoc.Find(teamCardBaseCountry).First().Text())

		var country, town string
		parts := strings.Split(townCountry, ",")
		if len(parts) >= 2 {
			town = strings.TrimSpace(parts[0])
			country = strings.TrimSpace(parts[1])
		} else if len(parts) == 1 {
			town = strings.TrimSpace(parts[0])
		}

		now := time.Now()
		team := &Team{
			ID:        uuid.New(),
			Name:      name,
			ShortName: shortName,
			Country:   country,
			Base:      town,
			CreatedAt: now,
			UpdatedAt: now,
		}

		teams = append(teams, team)
	})

	s.logger.Info("Scraping for teams completed.",
		slog.Int("totalParsed", len(teams)),
		slog.Duration("duration", time.Since(startTime)),
	)

	return teams, nil
}

func (s *ScrapedTeamService) UpdateTeamsFromScraper(ctx context.Context) error {
	scrapedTeams, err := s.ScrapeF1Teams(ctx)
	if err != nil {
		return err
	}

	for _, scrapedTeam := range scrapedTeams {
		existingTeams, err := s.teamRepository.FindAllByShortName(ctx, scrapedTeam.ShortName)
		if err != nil {
			s.logger.Error("Failed to query existing teams", slog.String("shortName", scrapedTeam.ShortName), slog.String("error", err.Error()))
			continue
		}

		if len(existingTeams) == 0 {
			if err := s.teamRepository.Save(ctx, scrapedTeam); err != nil {
				s.logger.Error("Failed to save new team", slog.String("name", scrapedTeam.Name), slog.String("error", err.Error()))
				continue
			}
			s.logger.Info("Saved new team", slog.String("name", scrapedTeam.Name))
		} else {
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
	}
	return nil
}
