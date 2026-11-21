package scrapers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"f1-api-go/internal/models"

	"github.com/google/uuid"
)

type jolpicaDriversResponse struct {
	MRData struct {
		DriverTable struct {
			Drivers []struct {
				DriverID        string `json:"driverId"`
				PermanentNumber string `json:"permanentNumber"`
				Code            string `json:"code"`
				GivenName       string `json:"givenName"`
				FamilyName      string `json:"familyName"`
				DateOfBirth     string `json:"dateOfBirth"`
				Nationality     string `json:"nationality"`
			} `json:"Drivers"`
		} `json:"DriverTable"`
	} `json:"MRData"`
}

type historicalDriverClient struct {
	httpClient *http.Client
	logger     *slog.Logger
}

func newHistoricalDriverClient(logger *slog.Logger) *historicalDriverClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &historicalDriverClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		logger:     logger,
	}
}

func (c *historicalDriverClient) fetchSeasonDrivers(ctx context.Context, year string) ([]*models.Driver, error) {
	url := "https://api.jolpi.ca/ergast/f1/drivers.json?limit=1000"
	if strings.TrimSpace(year) != "" {
		url = fmt.Sprintf("https://api.jolpi.ca/ergast/f1/%s/drivers.json?limit=100", strings.TrimSpace(year))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "f1-api-go/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jolpica drivers request failed (%s): status %d", url, resp.StatusCode)
	}

	var payload jolpicaDriversResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]*models.Driver, 0, len(payload.MRData.DriverTable.Drivers))
	for _, d := range payload.MRData.DriverTable.Drivers {
		first := strings.TrimSpace(d.GivenName)
		last := strings.TrimSpace(d.FamilyName)
		if first == "" && last == "" {
			continue
		}

		var bday time.Time
		if d.DateOfBirth != "" {
			if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(d.DateOfBirth)); err == nil {
				bday = parsed
			}
		}

		var permNum int
		if d.PermanentNumber != "" {
			if n, err := strconv.Atoi(strings.TrimSpace(d.PermanentNumber)); err == nil {
				permNum = n
			}
		}

		code := strings.ToUpper(strings.TrimSpace(d.Code))
		if len(code) != 3 && last != "" {
			folded := strings.ReplaceAll(normalizeText(last), " ", "")
			if len(folded) >= 3 {
				code = strings.ToUpper(folded[:3])
			}
		}

		nat := strings.TrimSpace(d.Nationality)
		normCountry := normalizeCountry(nat)
		out = append(out, &models.Driver{
			ID:              uuid.New(),
			Firstname:       first,
			Lastname:        last,
			DriverCode:      code,
			PermanentNumber: permNum,
			Birthday:        bday,
			Country:         normCountry,
			CountryCode:     getCountryCode(normCountry),
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	return out, nil
}

func (c *historicalDriverClient) lookup(ctx context.Context, year, code, firstName, lastName string) (*models.Driver, error) {
	candidates, err := c.fetchSeasonDrivers(ctx, year)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 && strings.TrimSpace(year) != "" {
		candidates, err = c.fetchSeasonDrivers(ctx, "")
		if err != nil {
			return nil, err
		}
	}

	normFirst := strings.ToLower(strings.TrimSpace(normalizeText(firstName)))
	normLast := strings.ToLower(strings.TrimSpace(normalizeText(lastName)))
	upperCode := strings.ToUpper(strings.TrimSpace(code))

	if normFirst != "" && normLast != "" {
		for _, cand := range candidates {
			if strings.ToLower(strings.TrimSpace(normalizeText(cand.Firstname))) == normFirst &&
				strings.ToLower(strings.TrimSpace(normalizeText(cand.Lastname))) == normLast {
				return cand, nil
			}
		}
	}

	if len(upperCode) == 3 {
		for _, cand := range candidates {
			if strings.ToUpper(strings.TrimSpace(cand.DriverCode)) == upperCode {
				return cand, nil
			}
		}
	}

	if normLast != "" {
		var surnameMatches []*models.Driver
		for _, cand := range candidates {
			if strings.ToLower(strings.TrimSpace(normalizeText(cand.Lastname))) == normLast {
				surnameMatches = append(surnameMatches, cand)
			}
		}
		if len(surnameMatches) == 1 {
			return surnameMatches[0], nil
		}
	}

	return nil, nil
}
