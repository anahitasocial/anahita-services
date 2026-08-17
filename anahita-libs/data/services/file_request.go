package services

type FileRequest struct {
	Read    *FileReadRequest
	Edit    *FileEditRequest
	Add     *FileAddRequest
	Delete  *FileDeleteRequest
	Exists  *FileExistsRequest
	Presign *FilePresignRequest
}

func NewFileRequest() *FileRequest {
	return &FileRequest{
		Read:    &FileReadRequest{},
		Edit:    &FileEditRequest{},
		Add:     &FileAddRequest{},
		Delete:  &FileDeleteRequest{},
		Exists:  &FileExistsRequest{},
		Presign: &FilePresignRequest{},
	}
}

type FileReadRequest struct {
	Path string `param:"path" validate:"required"`
}

type FileEditRequest struct {
	Path     string `validate:"required"`
	Content  []byte `validate:"required"`
	Mimetype string `validate:"required"`
	Size     int64  `validate:"required"`
}

type FileAddRequest struct {
	Path     string `validate:"required"`
	Content  []byte `validate:"required"`
	Mimetype string `validate:"required"`
	Size     int64  `validate:"required"`
}

type FileDeleteRequest struct {
	Path string `validate:"required"`
}

type FileExistsRequest struct {
	Path string `validate:"required"`
}

type FilePresignRequest struct {
	Path string `validate:"required"`
}
