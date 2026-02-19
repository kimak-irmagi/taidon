\echo 'Sakila sanity checks'
\c sakila
select count(*) as films from film;
select count(*) as customers from customer;

select f.title, c.name as category
from film f
join film_category fc on fc.film_id = f.film_id
join category c on c.category_id = fc.category_id
order by f.film_id
limit 10;
