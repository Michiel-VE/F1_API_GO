package models

type ScrapedStanding struct {
	Position    int    `json:"position"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DriverCode  string `json:"driver_code"`
	Nationality string `json:"nationality"`
	Team        string `json:"team"`
	Points      int    `json:"points"`
}