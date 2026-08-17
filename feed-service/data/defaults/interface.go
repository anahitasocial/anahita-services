package defaults

type DefaultLoader interface {
	Load() (*FeedDefaults, error)
}
