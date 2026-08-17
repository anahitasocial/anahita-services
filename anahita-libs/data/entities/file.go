package entities

type File struct {
	Content  []byte
	Path     string
	Mimetype string
	Size     int64
}
