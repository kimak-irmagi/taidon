\c sakila
select case when to_regclass('public.film') is null then 'missing' else 'ok' end;
select case when exists(select 1 from film) then 'ok' else 'empty' end;
