package constants

// Gender is on its way out — it drives no logic, and the request validator and
// the handler have never agreed on its values. See the person/user refactor
// plan; these remain only until the profile form stops sending the field.
const (
	UserGenderMale    = "male"
	UserGenderFemale  = "female"
	UserGenderNeutral = "neutral"
)

const UserSessionKey = "_anahita_session"
