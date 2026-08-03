package osmpass

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeName(t *testing.T) {
	assert.Equal(t, "col du telegraphe", normalizeName("Col du Télégraphe"))
	assert.Equal(t, "col de la gineste", normalizeName("Col de la Gineste"))
	assert.Equal(t, normalizeName("Col de la Gineste"), normalizeName("col de la gineste"))
	assert.True(t, nameMatches("Col de la Lombarde", "Col de la Lombarde / Colle della Lombarda"))
	assert.False(t, nameMatches("Col de la Gatasse", "Col de la Gatasso"))
}

func TestEnrichMountainPasses(t *testing.T) {
	db := testDB(t, `
		CREATE TABLE osm_passes (
			osm_id integer unique not null,
			name text,
			elevation integer,
			latitude real not null,
			longitude real not null
		);
		CREATE TABLE mountain_passes (
			external_id text unique not null,
			name text not null,
			department_code text not null,
			elevation integer not null,
			latitude real,
			longitude real
		);
	`)

	insert(t, db, "INSERT INTO osm_passes VALUES (1, 'Col de la Gineste', 327, 43.2, 5.4)")
	insert(t, db, "INSERT INTO osm_passes VALUES (2, 'Col de la Couillole', 1678, 44.1, 7.0)")
	insert(t, db, "INSERT INTO osm_passes VALUES (3, 'Col de la Gatasso', 122, 43.3, 5.5)")
	insert(t, db, "INSERT INTO osm_passes VALUES (4, 'Col des Portes', NULL, 43.6, 5.8)")

	insert(t, db, "INSERT INTO mountain_passes VALUES ('c/1', 'Col de la Gineste', '13', 326, NULL, NULL)")
	insert(t, db, "INSERT INTO mountain_passes VALUES ('c/2', 'Col de la Gatasse', '13', 120, NULL, NULL)")
	insert(t, db, "INSERT INTO mountain_passes VALUES ('c/3', 'Col inconnu', '13', 999, NULL, NULL)")

	count, err := EnrichMountainPasses(db)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	var latitude, longitude float64
	err = db.QueryRow("SELECT latitude, longitude FROM mountain_passes WHERE external_id = 'c/1'").Scan(&latitude, &longitude)
	require.NoError(t, err)
	assert.InDelta(t, 43.2, latitude, 1e-9)
	assert.InDelta(t, 5.4, longitude, 1e-9)

	err = db.QueryRow("SELECT latitude, longitude FROM mountain_passes WHERE external_id = 'c/2'").Scan(&latitude, &longitude)
	require.NoError(t, err)
	assert.InDelta(t, 43.3, latitude, 1e-9)
	assert.InDelta(t, 5.5, longitude, 1e-9)

	var isNull bool
	err = db.QueryRow("SELECT latitude IS NULL FROM mountain_passes WHERE external_id = 'c/3'").Scan(&isNull)
	require.NoError(t, err)
	assert.True(t, isNull)
}

func TestEnrichMountainPassesAmbiguousElevationOnly(t *testing.T) {
	db := testDB(t, `
		CREATE TABLE osm_passes (
			osm_id integer unique not null,
			name text,
			elevation integer,
			latitude real not null,
			longitude real not null
		);
		CREATE TABLE mountain_passes (
			external_id text unique not null,
			name text not null,
			department_code text not null,
			elevation integer not null,
			latitude real,
			longitude real
		);
	`)

	insert(t, db, "INSERT INTO osm_passes VALUES (1, 'Autre Col A', 122, 43.3, 5.5)")
	insert(t, db, "INSERT INTO osm_passes VALUES (2, 'Autre Col B', 130, 43.5, 5.7)")

	insert(t, db, "INSERT INTO mountain_passes VALUES ('c/1', 'Col de la Gatasse', '13', 120, NULL, NULL)")

	count, err := EnrichMountainPasses(db)
	require.NoError(t, err)
	assert.Zero(t, count)

	var isNull bool
	err = db.QueryRow("SELECT latitude IS NULL FROM mountain_passes WHERE external_id = 'c/1'").Scan(&isNull)
	require.NoError(t, err)
	assert.True(t, isNull)
}

func TestEnrichMountainPassesRejectsCrossDepartmentHomonym(t *testing.T) {
	db := testDB(t, `
		CREATE TABLE osm_passes (
			osm_id integer unique not null,
			name text,
			elevation integer,
			latitude real not null,
			longitude real not null
		);
		CREATE TABLE mountain_passes (
			external_id text unique not null,
			name text not null,
			department_code text not null,
			elevation integer not null,
			latitude real,
			longitude real
		);
	`)

	insert(t, db, "INSERT INTO osm_passes VALUES (1, 'Collet de la Selle', 1178, 43.77, 6.81)")

	insert(t, db, "INSERT INTO mountain_passes VALUES ('c/1', 'La Selle', '01', 1175, NULL, NULL)")

	count, err := EnrichMountainPasses(db)
	require.NoError(t, err)
	assert.Zero(t, count)

	var isNull bool
	err = db.QueryRow("SELECT latitude IS NULL FROM mountain_passes WHERE external_id = 'c/1'").Scan(&isNull)
	require.NoError(t, err)
	assert.True(t, isNull)
}

func testDB(t *testing.T, schema string) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(schema)
	require.NoError(t, err)
	return db
}

func insert(t *testing.T, db *sql.DB, query string) {
	_, err := db.Exec(query)
	require.NoError(t, err)
}
