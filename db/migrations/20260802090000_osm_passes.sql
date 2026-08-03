-- migrate:up
create table osm_passes (
    id integer primary key,
    osm_id integer unique not null,
    name text,
    elevation integer,
    latitude real not null,
    longitude real not null
);

alter table mountain_passes add column latitude real;
alter table mountain_passes add column longitude real;

-- migrate:down
alter table mountain_passes drop column latitude;
alter table mountain_passes drop column longitude;
drop table osm_passes;
