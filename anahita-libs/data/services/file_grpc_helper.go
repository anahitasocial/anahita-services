package services

import (
	"anahita-libs/data/entities"
	"log"
)

// PresignAvatar pre-signs the avatar URLs
func PresignAvatar(srv FilesGRPCInterface, item entities.AnahitaNodeAvatarInterface) {
	var (
		reqFile = NewFileRequest().Presign
	)

	avatars, err := item.GetAvatars()
	if err != nil {
		log.Println(err)
		return
	}

	for size, avatar := range avatars {
		if avatar.Url != "" {
			reqFile.Path = avatar.Url
			avatar.Url, err = srv.Presign(reqFile)
			if err != nil {
				log.Println(err)
			}

			// Update the avatar in the map using SetAvatar
			if err := item.SetAvatar(avatar, size); err != nil {
				log.Println("Error setting avatar:", err)
			}
		}
	}
}

// BatchPresignAvatar pre-signs the avatar URLs for a batch of items
func BatchPresignAvatar(srv FilesGRPCInterface, items []entities.AnahitaNodeAvatarInterface) {
	var paths []string
	var itemAvatarMap = make(map[string][]struct {
		item   entities.AnahitaNodeAvatarInterface
		size   string
		avatar entities.AnahitaNodeImage
	})

	// Collect all paths and track which items they belong to
	for _, item := range items {
		avatars, err := item.GetAvatars()
		if err != nil {
			log.Println(err)
			continue
		}

		for size, avatar := range avatars {
			if avatar.Url != "" {
				paths = append(paths, avatar.Url)
				itemAvatarMap[avatar.Url] = append(itemAvatarMap[avatar.Url], struct {
					item   entities.AnahitaNodeAvatarInterface
					size   string
					avatar entities.AnahitaNodeImage
				}{item, size, avatar})
			}
		}
	}

	// Batch presign all paths
	presignedUrls, err := srv.BatchPresign(paths)
	if err != nil {
		log.Println(err)
		return
	}

	// Update items with presigned URLs
	for i, path := range paths {
		if i < len(presignedUrls) {
			for _, mapping := range itemAvatarMap[path] {
				mapping.avatar.Url = presignedUrls[i]
				if err := mapping.item.SetAvatar(mapping.avatar, mapping.size); err != nil {
					log.Println("Error setting avatar:", err)
				}
			}
		}
	}
}

// PresignCover pre-signs the cover URLs
func PresignCover(srv FilesGRPCInterface, item entities.AnahitaNodeCoverInterface) {
	var (
		reqFile = NewFileRequest().Presign
	)

	covers, err := item.GetCovers()
	if err != nil {
		log.Println(err)
		return
	}

	for size, cover := range covers {
		if cover.Url != "" {
			reqFile.Path = cover.Url
			cover.Url, err = srv.Presign(reqFile)
			if err != nil {
				log.Println(err)
			}

			// Update the cover in the map using SetCover
			if err := item.SetCover(cover, size); err != nil {
				log.Println("Error setting cover:", err)
			}
		}
	}
}

// BatchPresignCover pre-signs the cover URLs for a batch of items
func BatchPresignCover(srv FilesGRPCInterface, items []entities.AnahitaNodeCoverInterface) {
	var paths []string
	var itemCoverMap = make(map[string][]struct {
		item  entities.AnahitaNodeCoverInterface
		size  string
		cover entities.AnahitaNodeImage
	})

	// Collect all paths and track which items they belong to
	for _, item := range items {
		covers, err := item.GetCovers()
		if err != nil {
			log.Println(err)
			continue
		}

		for size, cover := range covers {
			if cover.Url != "" {
				paths = append(paths, cover.Url)
				itemCoverMap[cover.Url] = append(itemCoverMap[cover.Url], struct {
					item  entities.AnahitaNodeCoverInterface
					size  string
					cover entities.AnahitaNodeImage
				}{item, size, cover})
			}
		}
	}

	// Batch presign all paths
	presignedUrls, err := srv.BatchPresign(paths)
	if err != nil {
		log.Println(err)
		return
	}

	// Update items with presigned URLs
	for i, path := range paths {
		if i < len(presignedUrls) {
			for _, mapping := range itemCoverMap[path] {
				mapping.cover.Url = presignedUrls[i]
				if err := mapping.item.SetCover(mapping.cover, mapping.size); err != nil {
					log.Println("Error setting cover:", err)
				}
			}
		}
	}
}

// PresignPortrait pre-signs the portrait URLs
func PresignPortrait(srv FilesGRPCInterface, item entities.AnahitaNodePortraitInterface) {
	var (
		reqFile = NewFileRequest().Presign
	)

	portraits, err := item.GetPortraits()
	if err != nil {
		log.Println(err)
		return
	}

	for size, portrait := range portraits {
		if portrait.Url != "" {
			reqFile.Path = portrait.Url
			portrait.Url, err = srv.Presign(reqFile)
			if err != nil {
				log.Println(err)
			}

			// Update the portrait in the map using SetPortrait
			if err := item.SetPortrait(portrait, size); err != nil {
				log.Println("Error setting portrait:", err)
			}
		}
	}
}

// BatchPresignPortrait pre-signs the portrait URLs for a batch of items
func BatchPresignPortrait(srv FilesGRPCInterface, items []entities.AnahitaNodePortraitInterface) {
	var paths []string
	var itemPortraitMap = make(map[string][]struct {
		item     entities.AnahitaNodePortraitInterface
		size     string
		portrait entities.AnahitaNodeImage
	})

	// Collect all paths and track which items they belong to
	for _, item := range items {
		portraits, err := item.GetPortraits()
		if err != nil {
			log.Println(err)
			continue
		}

		for size, portrait := range portraits {
			if portrait.Url != "" {
				paths = append(paths, portrait.Url)
				itemPortraitMap[portrait.Url] = append(itemPortraitMap[portrait.Url], struct {
					item     entities.AnahitaNodePortraitInterface
					size     string
					portrait entities.AnahitaNodeImage
				}{item, size, portrait})
			}
		}
	}

	// Batch presign all paths
	presignedUrls, err := srv.BatchPresign(paths)
	if err != nil {
		log.Println(err)
		return
	}

	// Update items with presigned URLs
	for i, path := range paths {
		if i < len(presignedUrls) {
			for _, mapping := range itemPortraitMap[path] {
				mapping.portrait.Url = presignedUrls[i]
				if err := mapping.item.SetPortrait(mapping.portrait, mapping.size); err != nil {
					log.Println("Error setting portrait:", err)
				}
			}
		}
	}
}

// BatchPresignNodes signs, for each node in a list, its avatar if it is an
// actor, its portrait if it is a medium, its cover, and its owner's and
// author's avatars. Nil entries are skipped. A service whose lists hold more
// (a post's parent, notifications, photo files) signs those itself.
func BatchPresignNodes(srv FilesGRPCInterface, items []*entities.AnahitaNode) {
	var avatarNodes []entities.AnahitaNodeAvatarInterface
	var coverNodes []entities.AnahitaNodeCoverInterface
	var portraitNodes []entities.AnahitaNodePortraitInterface
	var ownerAvatarNodes []entities.AnahitaNodeAvatarInterface
	var authorAvatarNodes []entities.AnahitaNodeAvatarInterface
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.AvatarFilename != "" && item.IsActor() {
			avatarNodes = append(avatarNodes, item)
		}
		if item.Filename != "" && item.IsMedium() {
			portraitNodes = append(portraitNodes, item)
		}
		if item.CoverFilename != "" {
			coverNodes = append(coverNodes, item)
		}
		if item.Owner != nil {
			ownerAvatarNodes = append(ownerAvatarNodes, item.Owner)
		}
		if item.Author != nil {
			authorAvatarNodes = append(authorAvatarNodes, item.Author)
		}
	}

	if len(avatarNodes) > 0 {
		BatchPresignAvatar(srv, avatarNodes)
	}
	if len(ownerAvatarNodes) > 0 {
		BatchPresignAvatar(srv, ownerAvatarNodes)
	}
	if len(authorAvatarNodes) > 0 {
		BatchPresignAvatar(srv, authorAvatarNodes)
	}
	if len(coverNodes) > 0 {
		BatchPresignCover(srv, coverNodes)
	}
	if len(portraitNodes) > 0 {
		BatchPresignPortrait(srv, portraitNodes)
	}
}
