package repositories

import (
	"anahita-libs/data/entities"
	"time"

	"anahita-libs/constants"
	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

// PersonInterface is the Person repository interface
type PersonInterface interface {
	// Browse returns a list of People
	Browse(ids []int64) ([]*entities.Person, error)

	// ReadByID returns a Person by id
	ReadByID(id int64) (*entities.Person, error)

	// ReadByEmail returns a Person by email
	ReadByEmail(string) (*entities.Person, error)

	// ReadByUsername returns a Person by Username
	ReadByUsername(string) (*entities.Person, error)

	// ReadByEmailOrUsername returns a Person by email or username
	ReadByEmailOrUsername(string) (*entities.Person, error)

	// Add creates a new Person
	Add(*entities.Person) (int64, error)

	// Edit updates a Person
	Edit(*entities.Person) error

	// Delete deletes a Person
	Delete(int64) error
}

// NewPerson creates a new Person repository
func NewPerson(db *sql.DB) PersonInterface {
	return &Person{
		entityType: constants.NodeActorPersonEntityType,
		db:         db,
		dbTimeout:  constants.DBTimeout,
	}
}

// Person is the Person repository
type Person struct {
	db         *sql.DB
	dbTimeout  time.Duration
	entityType string
}

func (repo *Person) getBaseSelectQuery() sq.SelectBuilder {
	var personFields = []string{
		"node_id",
		"username",
		"email",
		"usertype",
		"last_visit_date",
		"COALESCE(given_name, '') AS given_name",
		"COALESCE(family_name, '') AS family_name",
		"COALESCE(gender, '') AS gender",
	}

	return sq.Select(personFields...).From(
		"people_people AS person",
	)
}

// Browse returns a list of People
func (repo *Person) Browse(ids []int64) ([]*entities.Person, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery()
	query = query.Where(sq.Eq{"node_id": ids})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.Person
	for rows.Next() {
		item := entities.NewPerson()
		err := rows.Scan(
			&item.ID,
			&item.Username,
			&item.Email,
			&item.UserType,
			&item.LastVisitDate,
			&item.GivenName,
			&item.FamilyName,
			&item.Gender,
		)

		if err != nil {
			return nil, err
		}

		repo.afterRead(item)

		results = append(results, item)
	}

	return results, nil
}

// ReadByID returns a Person by id
func (repo *Person) ReadByID(id int64) (*entities.Person, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"node_id": id})
	return repo.getOneEntity(&query)
}

// ReadByEmail returns a Person by email
func (repo *Person) ReadByEmail(email string) (*entities.Person, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"email": email})
	return repo.getOneEntity(&query)
}

// ReadByUsername returns a Person by Username
func (repo *Person) ReadByUsername(username string) (*entities.Person, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"username": username})
	return repo.getOneEntity(&query)
}

// ReadByEmailOrUsername returns a Person by email or Username
func (repo *Person) ReadByEmailOrUsername(emailOrUsername string) (*entities.Person, error) {
	query := repo.getBaseSelectQuery().Where(sq.Or{
		sq.Eq{"email": emailOrUsername},
		sq.Eq{"username": emailOrUsername},
	})

	return repo.getOneEntity(&query)
}

// getOne returns a Person by query
func (repo *Person) getOneEntity(q *sq.SelectBuilder) (*entities.Person, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	sql, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewPerson()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	err = row.Scan(
		// person fields
		&item.ID,
		&item.Username,
		&item.Email,
		&item.UserType,
		&item.LastVisitDate,
		&item.GivenName,
		&item.FamilyName,
		&item.Gender,
	)

	if err != nil {
		return nil, err
	}

	repo.afterRead(item)

	return item, nil
}

// Add creates a new Person
func (repo *Person) Add(item *entities.Person) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	if err := item.Validate(); err != nil {
		return 0, err
	}

	query := `
		INSERT INTO people_people (
			node_id,
			Username,
			email,
			password,
			usertype,
			last_visit_date
		) VALUES (
			?,
			?,
			?,
			?,
			?,
			?
		)`

	_, err := repo.db.ExecContext(
		ctx,
		query,
		item.ID,
		item.Username,
		item.Email,
		item.Password,
		item.UserType,
		item.LastVisitDate,
	)
	if err != nil {
		return 0, err
	}

	return item.ID, nil
}

// Edit updates a Person
func (repo *Person) Edit(item *entities.Person) error {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	if err := item.Validate(); err != nil {
		return err
	}

	query := `
		UPDATE people_people
		SET
			given_name = ?,
			family_name = ?,
			gender = ?
		WHERE
			node_id = ?`

	_, err := repo.db.ExecContext(
		ctx,
		query,
		item.GivenName,
		item.FamilyName,
		item.Gender,
		item.ID,
	)

	if err != nil {
		return err
	}

	return nil
}

// Delete deletes a Person
func (repo *Person) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	query := `DELETE FROM people_people WHERE node_id = ?`
	_, err := repo.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}

// afterRead
func (repo *Person) afterRead(person *entities.Person) error {
	person.GetAvatars()
	person.GetCovers()
	person.GetInformation([]string{})

	return nil
}
