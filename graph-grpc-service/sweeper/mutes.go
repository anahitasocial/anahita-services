package sweeper

import (
	"log"

	"graph/data/repositories"
)

// Mutes deletes the mutes that have ended (roadmap row 133). Nobody is told:
// a mute is the muter's alone, and it stopped counting when it ended.
func Mutes(repos *repositories.Repositories) int64 {
	n, err := repos.SocialgraphRel.DeleteEndedMutes()
	if err != nil {
		log.Printf("sweeper: mutes: %v", err)
		return 0
	}
	if n > 0 {
		log.Printf("sweeper: %d mute(s) ended", n)
	}
	return n
}
