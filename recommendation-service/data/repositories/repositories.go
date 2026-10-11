package repositories

import "database/sql"

type Repositories struct {
	Signals     SignalsInterface
	Preferences PreferencesInterface
	Dismissals  DismissalsInterface
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		Signals:     NewSignals(db),
		Preferences: NewPreferences(db),
		Dismissals:  NewDismissals(db),
	}
}
