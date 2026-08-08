-- migrate:up
alter table rides add column cotacol_score real;
alter table rides add column cotacol_algo_version text;

-- migrate:down
alter table rides drop column cotacol_algo_version;
alter table rides drop column cotacol_score;
