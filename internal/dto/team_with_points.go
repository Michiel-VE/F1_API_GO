package dto

import "github.com/google/uuid"

type TeamWithPoints interface {
	GetID() uuid.UUID
	GetName() string
	GetShortName() string
	GetCountry() string
	GetBase() string
	GetTotalPoints() int
}

type TeamWithPointsDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	ShortName   string    `json:"short_name"`
	Country     string    `json:"country"`
	Base        string    `json:"base"`
	TotalPoints int       `json:"total_points"`
}

func (t TeamWithPointsDTO) GetID() uuid.UUID     { return t.ID }
func (t TeamWithPointsDTO) GetName() string      { return t.Name }
func (t TeamWithPointsDTO) GetShortName() string { return t.ShortName }
func (t TeamWithPointsDTO) GetCountry() string   { return t.Country }
func (t TeamWithPointsDTO) GetBase() string      { return t.Base }
func (t TeamWithPointsDTO) GetTotalPoints() int  { return t.TotalPoints }