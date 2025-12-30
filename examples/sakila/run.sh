sqlrs -from postgres:17 \
  --storage btrfs \
  --snapshots stop \
  --prepare examples/sakila/prepare.sql \
  -run -- psql -f examples/sakila/queries.sql
