\echo 'PostgresPro demo sanity checks'
select count(*) as airports from airports;
select count(*) as flights from flights;

select flight_no, scheduled_departure, scheduled_arrival
from flights
order by scheduled_departure desc
limit 10;
