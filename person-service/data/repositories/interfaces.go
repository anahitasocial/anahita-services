package repositories

type TaggableNodeInterface interface {
	GetID() int64
	GetService() string
	GetBody() string
	GetType() string
	GetEditorID() int64
}
