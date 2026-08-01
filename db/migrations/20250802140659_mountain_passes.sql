-- migrate:up
create table mountain_passes (
    id integer primary key,
    external_id text unique not null,
    name text not null,
    country_code text not null,
    department_code text not null,
    elevation integer not null
);

-- migrate:down
drop table mountain_passes;
