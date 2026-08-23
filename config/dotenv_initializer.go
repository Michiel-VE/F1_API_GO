package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AppConfig struct {
	DatabaseURL        string
	DatabaseUser       string
	DatabasePassword   string
	GoogleClientID     string
	GoogleClientSecret string
	JWTSecretKey       string
	RedisURL           string

	ServerPort          string
	GoogleOAuth         *oauth2.Config
	GoogleUserInfoURI   string
	GoogleJWKSetURI     string
	GoogleUserNameAttr  string
	JwtExpiration       time.Duration
	DatabaseMinIdle     string
	DatabaseMaxPoolSize string
	DatabaseIdleTimeout time.Duration
	DatabaseMaxLifetime time.Duration
	DatabaseTimeout     time.Duration
	RedisCacheTTL       time.Duration
	RedisCacheNulls     bool
	CacheEnabled        bool
}

var (
	AppCfg AppConfig
	once   sync.Once
)

func InitConfig() {
	once.Do(func() {
		env := os.Getenv("ENV")

		if strings.EqualFold(env, "prod") {
			loadSecretsManager()
		} else {
			loadLocalEnv()
		}
	})
}

func buildGoogleOAuth(clientID, clientSecret string) *oauth2.Config {
	redirectURI := os.Getenv("SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_GOOGLE_REDIRECT_URI")
	if redirectURI == "" {
		baseURL := getEnv("BASE_URL", "http://localhost:8080")
		redirectURI = baseURL + "/login/oauth2/code/google"
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint:     google.Endpoint,
	}
}

// cacheEnabledForLocal defaults to false for local dev so responses always
// reflect the latest code/DB state without manual cache flushing. Set
// CACHE_ENABLED=true in local.env to opt back into caching locally.
func cacheEnabledForLocal() bool {
	val := os.Getenv("CACHE_ENABLED")
	if val == "" {
		return false
	}
	enabled, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return enabled
}

func loadLocalEnv() {
	log.Println("Loading local environment variables...")
	_ = godotenv.Load("local.env")

	rawRedisURL := os.Getenv("UPSTASH_REDIS_URL")
	redisToken := os.Getenv("UPSTASH_REDIS_TOKEN")

	var redisURL string
	if rawRedisURL != "" && redisToken != "" {
		endpoint := strings.TrimPrefix(rawRedisURL, "https://")
		redisURL = fmt.Sprintf("rediss://default:%s@%s:6379", redisToken, endpoint)
	} else {
		redisURL = os.Getenv("REDIS_URL")
	}

	dbUser := os.Getenv("DATABASE_USER")
	dbPassword := os.Getenv("DATABASE_PASSWORD")
	dbHost := os.Getenv("DATABASE_URL")

	var constructedURL string
	if dbUser != "" && dbPassword != "" && dbHost != "" {
		constructedURL = fmt.Sprintf(
			"postgres://%s:%s@%s:6543/postgres?sslmode=require",
			dbUser,
			dbPassword,
			dbHost,
		)
	} else {
		constructedURL = os.Getenv("SPRING_DATASOURCE_URL")
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	AppCfg = AppConfig{
		DatabaseURL:        constructedURL,
		DatabaseUser:       dbUser,
		DatabasePassword:   dbPassword,
		GoogleClientID:     clientID,
		GoogleClientSecret: clientSecret,
		JWTSecretKey:       os.Getenv("JWT_SECRET_KEY"),
		RedisURL:           redisURL,

		ServerPort:          getEnv("PORT", "8080"),
		GoogleOAuth:         buildGoogleOAuth(clientID, clientSecret),
		GoogleUserInfoURI:   "https://www.googleapis.com/oauth2/v3/userinfo",
		GoogleJWKSetURI:     "https://www.googleapis.com/oauth2/v3/certs",
		GoogleUserNameAttr:  "sub",
		JwtExpiration:       3600 * time.Second,
		DatabaseMinIdle:     "1",
		DatabaseMaxPoolSize: "2",
		DatabaseIdleTimeout: 30 * time.Second,
		DatabaseMaxLifetime: 60 * time.Second,
		DatabaseTimeout:     15 * time.Second,
		RedisCacheTTL:       3600 * time.Second,
		RedisCacheNulls:     false,
		CacheEnabled:        cacheEnabledForLocal(),
	}
}

func loadSecretsManager() {
	appSecretName := os.Getenv("APP_SECRET_NAME")
	if strings.TrimSpace(appSecretName) == "" {
		log.Fatal("Missing APP_SECRET_NAME environment variable.")
	}

	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion("eu-north-1"))
	if err != nil {
		log.Fatalf("Unable to load AWS SDK config: %v", err)
	}

	client := secretsmanager.NewFromConfig(cfg)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(appSecretName),
	}

	result, err := client.GetSecretValue(context.TODO(), input)
	if err != nil {
		log.Fatalf("Failed to load secrets: %v", err)
	}

	var secrets map[string]string
	if err := json.Unmarshal([]byte(*result.SecretString), &secrets); err != nil {
		log.Fatalf("Failed to parse secret JSON: %v", err)
	}

	token := secrets["upstash_redis_token"]
	endpoint := strings.TrimPrefix(secrets["upstash_redis_url"], "https://")
	redisURL := fmt.Sprintf("rediss://default:%s@%s:6379", token, endpoint)

	dbUser := secrets["db_username"]
	dbPassword := secrets["db_password"]
	dbHost := secrets["url"]

	var constructedURL string
	if dbUser != "" && dbPassword != "" && dbHost != "" {
		constructedURL = fmt.Sprintf(
			"postgres://%s:%s@%s:6543/postgres?sslmode=require",
			dbUser,
			dbPassword,
			dbHost,
		)
	}

	clientID := secrets["google_client_id"]
	clientSecret := secrets["google_client_secret"]

	AppCfg = AppConfig{
		DatabaseURL:        constructedURL,
		DatabaseUser:       dbUser,
		DatabasePassword:   dbPassword,
		GoogleClientID:     clientID,
		GoogleClientSecret: clientSecret,
		JWTSecretKey:       secrets["jwt_secret_key"],
		RedisURL:           redisURL,

		ServerPort:          getEnv("PORT", "8080"),
		GoogleOAuth:         buildGoogleOAuth(clientID, clientSecret),
		GoogleUserInfoURI:   "https://www.googleapis.com/oauth2/v3/userinfo",
		GoogleJWKSetURI:     "https://www.googleapis.com/oauth2/v3/certs",
		GoogleUserNameAttr:  "sub",
		JwtExpiration:       3600 * time.Second,
		DatabaseMinIdle:     "1",
		DatabaseMaxPoolSize: "2",
		DatabaseIdleTimeout: 30 * time.Second,
		DatabaseMaxLifetime: 60 * time.Second,
		DatabaseTimeout:     15 * time.Second,
		RedisCacheTTL:       3600 * time.Second,
		RedisCacheNulls:     false,
		CacheEnabled:        true,
	}

	log.Printf("All secrets loaded from AWS Secret (%s) successfully.\n", appSecretName)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
