package entities

type ActorRelation struct {
	ActorAId    int64
	ActorBId    int64
	IsLeading   bool // actorB -> actorA (actorB follows actorA)
	IsFollowing bool // actorA -> actorB (actorA follows actorB)
	IsMutual    bool // IsLeading && IsFollowing
	IsBlocking  bool // actorA blocks actorB
	HasAdmin    bool // actorA has actorB as admin
	// actorB has asked to follow actorA, and has not been answered
	HasRequested bool
}
