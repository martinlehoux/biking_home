CREATE TABLE IF NOT EXISTS "schema_migrations" (version varchar(255) primary key);
CREATE TABLE mountain_passes (
    id integer primary key,
    external_id text unique not null,
    name text not null,
    country_code text not null,
    department_code text not null,
    elevation integer not null
, latitude real, longitude real);
CREATE TABLE osm_passes (
    id integer primary key,
    osm_id integer unique not null,
    name text,
    elevation integer,
    latitude real not null,
    longitude real not null
);
CREATE TABLE rides (
    id integer primary key,
    external_id text unique not null,
    gpx_path text not null,
    name text not null,
    type text not null,
    start_date text not null,
    distance_m real not null,
    moving_time_s integer not null,
    elapsed_time_s integer not null,
    total_elevation_gain_m real not null,
    average_speed_mps real not null,
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at text not null default (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
-- Dbmate schema migrations
INSERT INTO "schema_migrations" (version) VALUES
  ('20250802140659'),
  ('20260802090000'),
  ('20260804000000');
