\c chinook
select case when to_regclass('public.album') is null then 'missing' else 'ok' end;
select case when exists(select 1 from album) then 'ok' else 'empty' end;
