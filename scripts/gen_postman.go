package main

import (
	"encoding/json"
	"os"
	"strings"

	"f1-api-go/config"
	"f1-api-go/controller"
	"f1-api-go/services"
)

type PostmanCollection struct {
	Info PostmanInfo   `json:"info"`
	Item []interface{} `json:"item"`
}

type PostmanInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

type PostmanFolder struct {
	Name string        `json:"name"`
	Item []PostmanItem `json:"item"`
}

type PostmanItem struct {
	Name    string         `json:"name"`
	Request PostmanRequest `json:"request"`
}

type PostmanRequest struct {
	Method string          `json:"method"`
	Header []PostmanHeader `json:"header,omitempty"`
	Body   *PostmanBody    `json:"body,omitempty"`
	URL    PostmanURL      `json:"url"`
}

type PostmanHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

type PostmanBody struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

type PostmanURL struct {
	Raw  string   `json:"raw"`
	Host []string `json:"host"`
	Path []string `json:"path"`
}

func extractFolder(path string) string {
	cleanPath := strings.TrimPrefix(path, "/")
	parts := strings.Split(cleanPath, "/")

	var rawFolder string
	if len(parts) >= 3 && parts[0] == "api" {
		rawFolder = parts[2]
	} else if len(parts) > 0 && parts[0] != "" {
		rawFolder = parts[0]
	} else {
		rawFolder = "General"
	}

	rawFolder = strings.ToLower(rawFolder)

	switch rawFolder {
	case "driver":
		return "Drivers"
	case "team":
		return "Teams"
	case "race":
		return "Races"
	case "prediction":
		return "Predictions"
	case "pool", "pools":
		return "Predictions"
	case "auth", "oauth2", "login":
		return "Auth"
	case "profile":
		return "Profile"
	}

	return capitalize(rawFolder)
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func getSampleBody(path string, method string) *PostmanBody {
	if method != "POST" && method != "PUT" {
		return nil
	}

	var sample string
	if strings.Contains(path, "/auth/login") || strings.Contains(path, "/auth/register") {
		sample = "{\n  \"email\": \"user@example.com\",\n  \"password\": \"your_password\"\n}"
	} else if strings.Contains(path, "/predictions/pools/join") {
		sample = "{\n  \"inviteCode\": \"ABC123\"\n}"
	} else if strings.Contains(path, "/predictions/pools") {
		sample = "{\n  \"name\": \"My F1 League\"\n}"
	} else if strings.Contains(path, "/predictions") {
		sample = "{\n  \"season\": 2026,\n  \"raceId\": 1\n}"
	} else {
		sample = "{\n  \n}"
	}

	return &PostmanBody{
		Mode: "raw",
		Raw:  sample,
	}
}

func main() {
	jwtService, _ := services.NewJWTService("dummy-secret-key-for-gen", 3600)

	r := config.SecurityConfig(
		&controller.AuthController{},
		&controller.ProfileController{},
		&controller.DriverController{},
		&controller.TeamController{},
		&controller.RaceController{},
		&controller.PredictionController{},
		jwtService,
		nil,
	)

	collection := PostmanCollection{
		Info: PostmanInfo{
			Name:   "F1 API Local (Auto-Generated)",
			Schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		Item: []interface{}{},
	}

	baseURL := "http://localhost:8080"

	folderMap := make(map[string][]PostmanItem)
	var folderOrder []string

	for _, route := range r.Routes() {
		if route.Path == "/no-route-fallback" {
			continue
		}

		folderName := extractFolder(route.Path)

		var pathSegments []string
		for _, segment := range strings.Split(route.Path, "/") {
			if segment != "" {
				pathSegments = append(pathSegments, segment)
			}
		}

		// Only standard headers (Content-Type)
		headers := []PostmanHeader{
			{Key: "Content-Type", Value: "application/json", Type: "text"},
		}

		item := PostmanItem{
			Name: route.Method + " " + route.Path,
			Request: PostmanRequest{
				Method: route.Method,
				Header: headers,
				Body:   getSampleBody(route.Path, route.Method),
				URL: PostmanURL{
					Raw:  baseURL + route.Path,
					Host: []string{"localhost:8080"},
					Path: pathSegments,
				},
			},
		}

		if _, exists := folderMap[folderName]; !exists {
			folderOrder = append(folderOrder, folderName)
		}
		folderMap[folderName] = append(folderMap[folderName], item)
	}

	for _, folderName := range folderOrder {
		collection.Item = append(collection.Item, PostmanFolder{
			Name: folderName,
			Item: folderMap[folderName],
		})
	}

	file, err := os.Create("scripts/f1-api.postman_collection.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(collection); err != nil {
		panic(err)
	}
}
