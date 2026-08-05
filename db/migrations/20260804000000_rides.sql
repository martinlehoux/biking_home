-- migrate:up
create table rides (
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

-- migrate:down
drop table rides;
