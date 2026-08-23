package dto

type DriverRaceResultsResponse struct {
	Firstname string          `json:"firstname"`
	Lastname  string          `json:"lastname"`
	Season    string          `json:"season"`
	Results   []RaceResultDTO `json:"results"`
}