package resolvers

type RaceCacheResolver struct {
	BaseSeasonAwareCacheResolver
}

func NewRaceCacheResolver() *RaceCacheResolver {
	return &RaceCacheResolver{
		BaseSeasonAwareCacheResolver: BaseSeasonAwareCacheResolver{
			ActiveName:     "races",
			HistoricalName: "races_hist",
		},
	}
}