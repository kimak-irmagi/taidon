\echo 'PostgresPro demo sanity checks'
\connect demo
SET search_path = bookings, pg_catalog;

select count(*) as airports from airports;
select count(*) as flights from flights;

select flight_no, scheduled_departure, scheduled_arrival
from flights
order by scheduled_departure desc
limit 10;
