package config

import (
	"strconv"

	"gorm.io/gorm"
)

func atoiOr(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// ApplyDBPool applies the pool settings that AppConfig already defined
// (DatabaseMaxPoolSize, DatabaseMinIdle, DatabaseIdleTimeout, DatabaseMaxLifetime).
// They were Spring/Hikari leftovers that nothing ever read, so the connection
// pool ran with Go's defaults: unlimited open connections and no recycling.
// Recycling matters on Lambda: a frozen container comes back with connections
// the pooler has already closed.
func ApplyDBPool(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	maxOpen := atoiOr(AppCfg.DatabaseMaxPoolSize, 2)
	maxIdle := atoiOr(AppCfg.DatabaseMinIdle, 1)
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxIdleTime(AppCfg.DatabaseIdleTimeout)
	sqlDB.SetConnMaxLifetime(AppCfg.DatabaseMaxLifetime)
	return nil
}
