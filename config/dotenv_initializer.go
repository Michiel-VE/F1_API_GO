package config

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
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
	ResendAPIKey       string
	AlertEmailFrom     string
	AlertEmailTo       string

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

// IsProduction is the one place that decides "are we in production". It uses
// the same ENV variable that picks Secrets Manager over local.env.
func IsProduction() bool {
	return strings.EqualFold(os.Getenv("ENV"), "prod")
}

func InitConfig() {
	once.Do(func() {
		if IsProduction() {
			loadSecretsManager()
		} else {
			loadLocalEnv()
		}
	})
}

// postgresURL builds the connection string with proper escaping. The old
// fmt.Sprintf broke as soon as the password contained @ : / ? # or %.
func postgresURL(user, password, host string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     host + ":6543",
		Path:     "/postgres",
		RawQuery: "sslmode=require",
	}
	return u.String()
}

// upstashRedisURL builds the rediss:// URL with the token escaped.
func upstashRedisURL(rawEndpoint, token string) string {
	host := strings.TrimSpace(rawEndpoint)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimSuffix(host, "/")
	u := url.URL{
		Scheme: "rediss",
		User:   url.UserPassword("default", token),
		Host:   host + ":6379",
	}
	return u.String()
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
		redisURL = upstashRedisURL(rawRedisURL, redisToken)
	} else {
		redisURL = os.Getenv("REDIS_URL")
	}

	dbUser := os.Getenv("DATABASE_USER")
	dbPassword := os.Getenv("DATABASE_PASSWORD")
	dbHost := os.Getenv("DATABASE_URL")

	var constructedURL string
	if dbUser != "" && dbPassword != "" && dbHost != "" {
		constructedURL = postgresURL(dbUser, dbPassword, dbHost)
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
		ResendAPIKey:       os.Getenv("RESEND_API_KEY"),
		AlertEmailFrom:     os.Getenv("ALERT_EMAIL_FROM"),
		AlertEmailTo:       os.Getenv("ALERT_EMAIL_TO"),

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

	// Without a timeout, a hanging Secrets Manager call would hang the whole
	// cold start until the Lambda itself times out.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion("eu-north-1"))
	if err != nil {
		log.Fatalf("Unable to load AWS SDK config: %v", err)
	}

	client := secretsmanager.NewFromConfig(cfg)
	result, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(appSecretName),
	})
	if err != nil {
		log.Fatalf("Failed to load secrets: %v", err)
	}
	if result.SecretString == nil {
		log.Fatalf("Secret %s has no string value", appSecretName)
	}

	var secrets map[string]string
	if err := json.Unmarshal([]byte(*result.SecretString), &secrets); err != nil {
		log.Fatalf("Failed to parse secret JSON: %v", err)
	}

	// Fail at startup with a clear message instead of later with a confusing
	// database or Redis error.
	var missing []string
	for _, key := range []string{"db_username", "db_password", "url", "jwt_secret_key", "upstash_redis_token", "upstash_redis_url"} {
		if strings.TrimSpace(secrets[key]) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		log.Fatalf("Secret %s is missing required keys: %s", appSecretName, strings.Join(missing, ", "))
	}

	dbUser := secrets["db_username"]
	dbPassword := secrets["db_password"]
	clientID := secrets["google_client_id"]
	clientSecret := secrets["google_client_secret"]

	AppCfg = AppConfig{
		DatabaseURL:        postgresURL(dbUser, dbPassword, secrets["url"]),
		DatabaseUser:       dbUser,
		DatabasePassword:   dbPassword,
		GoogleClientID:     clientID,
		GoogleClientSecret: clientSecret,
		JWTSecretKey:       secrets["jwt_secret_key"],
		RedisURL:           upstashRedisURL(secrets["upstash_redis_url"], secrets["upstash_redis_token"]),
		ResendAPIKey:       secrets["resend_api_key"],
		AlertEmailFrom:     secrets["alert_email_from"],
		AlertEmailTo:       secrets["alert_email_to"],

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
