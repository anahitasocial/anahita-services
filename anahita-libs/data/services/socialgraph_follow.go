package services

import "anahita-libs/data/entities"

type SocialgraphFollowRequest struct {
	Edit   *SocialgraphFollowEditRequest
	Add    *SocialgraphFollowAddRequest
	Delete *SocialgraphFollowDeleteRequest
}

func NewSocialgraphFollowRequest() *SocialgraphFollowRequest {
	return &SocialgraphFollowRequest{
		Edit: &SocialgraphFollowEditRequest{
			AcceptFollowRequest: false,
		},
		Add: &SocialgraphFollowAddRequest{
			IsFollowRequest: false,
		},
		Delete: &SocialgraphFollowDeleteRequest{},
	}
}

type SocialgraphFollowEditRequest struct {
	Source              *entities.AnahitaNode
	Target              *entities.AnahitaNode
	Viewer              *entities.Viewer
	AcceptFollowRequest bool
}

type SocialgraphFollowAddRequest struct {
	Source          *entities.AnahitaNode
	Target          *entities.AnahitaNode
	Viewer          *entities.Viewer
	IsFollowRequest bool
}

type SocialgraphFollowDeleteRequest struct {
	Source *entities.AnahitaNode
	Target *entities.AnahitaNode
	Viewer *entities.Viewer
}
