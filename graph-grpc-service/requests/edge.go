package requests

type EdgeRequest struct {
	Browse               *EdgeBrowseRequest
	ReadByFilters        *EdgeReadByFiltersRequest
	BatchRead            *EdgeBatchReadRequest
	BatchReadByFilters   *EdgeBatchReadByFiltersRequest
	BatchDeleteByFilters *EdgeBatchDeleteByFiltersRequest
	Count                *EdgeCountRequest
}

func NewEdgeRequest() *EdgeRequest {
	return &EdgeRequest{
		Browse: &EdgeBrowseRequest{
			Offset:      0,
			Limit:       20,
			OrderingDir: "desc",
			Ordering:    "ordering",
		},
		ReadByFilters: &EdgeReadByFiltersRequest{},
		BatchRead: &EdgeBatchReadRequest{
			IDs:      []int64{},
			Ordering: "created_at",
		},
		BatchReadByFilters: &EdgeBatchReadByFiltersRequest{
			Ordering: "created_at",
		},
		BatchDeleteByFilters: &EdgeBatchDeleteByFiltersRequest{},
		Count:                &EdgeCountRequest{},
	}
}

type EdgeBrowseRequest struct {
	NodeAID     int64    `validate:"omitempty,numeric"`
	NodeAType   string   `validate:"omitempty,max=50"`
	Types       []string `validate:"max=50"`
	NodeBID     int64    `validate:"omitempty,numeric"`
	NodeBType   string   `validate:"omitempty,max=50"`
	Limit       uint64   `validate:"omitempty,numeric,max=100"`
	Offset      uint64   `validate:"omitempty,numeric"`
	Ordering    string   `validate:"max=10,omitempty,oneof=created_at updated_at ordering"`
	OrderingDir string   `validate:"max=4,omitempty,oneof=asc desc"`
}

type EdgeReadByFiltersRequest struct {
	NodeAID   int64    `validate:"omitempty,numeric"`
	NodeAType string   `validate:"omitempty,max=50"`
	Types     []string `validate:"max=50"`
	NodeBID   int64    `validate:"omitempty,numeric"`
	NodeBType string   `validate:"omitempty,max=50"`
}

type EdgeBatchReadRequest struct {
	IDs      []int64 `validate:"required,min=1,dive,required,numeric"`
	Ordering string  `validate:"max=10,omitempty,oneof=created_at updated_at ordering"`
}

type EdgeBatchReadByFiltersRequest struct {
	NodeAID     int64    `validate:"omitempty,numeric"`
	NodeAType   string   `validate:"omitempty,max=50"`
	Types       []string `validate:"max=50"`
	NodeBID     int64    `validate:"omitempty,numeric"`
	NodeBType   string   `validate:"omitempty,max=50"`
	Ordering    string   `validate:"max=10,omitempty,oneof=created_at updated_at ordering"`
	OrderingDir string   `validate:"max=4,omitempty,oneof=asc desc"`
}

type EdgeBatchDeleteByFiltersRequest struct {
	NodeAID   int64    `validate:"omitempty,numeric"`
	NodeAType string   `validate:"omitempty,max=50"`
	Types     []string `validate:"max=50"`
	NodeBID   int64    `validate:"omitempty,numeric"`
	NodeBType string   `validate:"omitempty,max=50"`
}

type EdgeCountRequest struct {
	NodeAID   int64    `validate:"omitempty,numeric"`
	NodeAType string   `validate:"omitempty,max=50"`
	Types     []string `validate:"max=50"`
	NodeBID   int64    `validate:"omitempty,numeric"`
	NodeBType string   `validate:"omitempty,max=50"`
}
