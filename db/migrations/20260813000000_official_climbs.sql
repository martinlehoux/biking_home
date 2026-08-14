-- migrate:up
create table official_climbs (
    id integer primary key,
    name text not null,
    start_latitude real not null,
    start_longitude real not null,
    end_latitude real not null,
    end_longitude real not null,
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at text not null default (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- migrate:down
drop table official_climbs;
