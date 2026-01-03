\echo 'Chinook sanity checks'
\c chinook
select count(*) as albums from album;
select count(*) as tracks from track;
select a.title, ar.name
from album a
join artist ar on ar.artist_id = a.artist_id
order by a.album_id
limit 10;
