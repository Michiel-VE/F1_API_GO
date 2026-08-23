package resolvers

type ResultsCacheResolver struct {
	BaseSeasonAwareCacheResolver
}

func NewResultsCacheResolver() *ResultsCacheResolver {
	return &ResultsCacheResolver{
		BaseSeasonAwareCacheResolver: BaseSeasonAwareCacheResolver{
			ActiveName:     "results",
			HistoricalName: "results_hist",
		},
	}
}