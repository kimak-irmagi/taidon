\c postgres
select case when to_regclass('public.databasechangelog') is null then 'missing' else 'ok' end;
select case when to_regclass('public.jhi_user') is null then 'missing' else 'ok' end;
select case when exists(select 1 from public.jhi_user) then 'ok' else 'empty' end;