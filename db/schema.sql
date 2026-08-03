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
-- Dbmate schema migrations
INSERT INTO "schema_migrations" (version) VALUES
  ('20250802140659'),
  ('20260802090000');
