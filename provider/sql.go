package provider

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/cznic/ql"
	"github.com/prometheus/common/model"

	"github.com/prometheus/alertmanager/types"
)

func init() {
	ql.RegisterDriver()
}

type SQLSilences struct {
	db *sql.DB
}

func (s *SQLSilences) Close() error {
	return s.db.Close()
}

func NewSQLSilences() (*SQLSilences, error) {
	db, err := sql.Open("ql", "data/am.db")
	if err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(createSilencesTable); err != nil {
		tx.Rollback()
		return nil, err
	}
	tx.Commit()

	return &SQLSilences{db: db}, nil
}

const createSilencesTable = `
CREATE TABLE IF NOT EXISTS silences (
	matchers   string,
	start_at   time,
	ends_at    time,
	created_at time,
	created_by string,
	comment    string
);
CREATE INDEX IF NOT EXISTS silences_end ON silences (ends_at);
CREATE INDEX IF NOT EXISTS silences_id  ON silences (id());
`

func (s *SQLSilences) Mutes(lset model.LabelSet) bool {
	return false
}

func (s *SQLSilences) All() ([]*types.Silence, error) {
	rows, err := s.db.Query(`SELECT id(), matchers, start_at, ends_at, created_at, created_by, comment FROM silences`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var silences []*types.Silence

	for rows.Next() {
		var (
			createdAt time.Time
			sil       types.Silence
			matchers  string
		)

		if err := rows.Scan(&sil.ID, &matchers, &sil.StartsAt, &sil.EndsAt, &createdAt, &sil.CreatedBy, &sil.Comment); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(matchers), &sil.Matchers); err != nil {
			return nil, err
		}

		silences = append(silences, &sil)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return silences, nil
}

func (s *SQLSilences) Set(sil *types.Silence) (uint64, error) {
	mb, err := json.Marshal(sil.Matchers)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}

	res, err := tx.Exec(`INSERT INTO silences VALUES ($1, $2, $3, $3, $5, $6)`,
		string(mb),
		sil.StartsAt,
		sil.EndsAt,
		time.Time{},
		sil.CreatedBy,
		sil.Comment,
	)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	sid, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	tx.Commit()

	return uint64(sid), nil
}

func (s *SQLSilences) Del(uint64) error {
	return nil
}

func (s *SQLSilences) Get(uint64) (*types.Silence, error) {
	return nil, nil
}