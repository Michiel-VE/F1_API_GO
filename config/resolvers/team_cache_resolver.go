package resolvers

type TeamCacheResolver struct {
	BaseSeasonAwareCacheResolver
}

func NewTeamCacheResolver() *TeamCacheResolver {
	return &TeamCacheResolver{
		BaseSeasonAwareCacheResolver: BaseSeasonAwareCacheResolver{
			ActiveName:     "teams",
			HistoricalName: "teams_hist",
		},
	}
}