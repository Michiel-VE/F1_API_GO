package resolvers

type DriverCacheResolver struct {
	BaseSeasonAwareCacheResolver
}

func NewDriverCacheResolver() *DriverCacheResolver {
	return &DriverCacheResolver{
		BaseSeasonAwareCacheResolver: BaseSeasonAwareCacheResolver{
			ActiveName:     "drivers",
			HistoricalName: "drivers_hist",
		},
	}
}