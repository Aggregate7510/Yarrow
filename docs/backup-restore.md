# Backup and restore

All of Yarrow's state is one SQLite database, `/data/storage.db`, in the `yarrow_yarrow-data` Docker volume.

## Back up (while running)

```sh
cd ~/yarrow
docker compose exec yarrow yarrow backup -to /data/backups/storage-$(date +%F).db
docker cp yarrow:/data/backups/storage-$(date +%F).db ~/yarrow-backups/
```

`yarrow backup` uses SQLite's `VACUUM INTO`, which produces a consistent, compacted snapshot even while the server is writing. It checks the copy's integrity and prints its article counts. Copying `storage.db` with `cp` while Yarrow runs is **not** safe because of the write-ahead log.

To back up every night, add to the host's crontab (`crontab -e`):

```
30 3 * * * cd /home/pi/yarrow && docker compose exec -T yarrow yarrow backup -to /data/backups/storage-$(date +\%F).db >/dev/null && docker cp yarrow:/data/backups/storage-$(date +\%F).db /home/pi/yarrow-backups/
```

Backups inside the volume accumulate; prune them occasionally, e.g. `docker compose exec yarrow sh -c 'ls -t /data/backups/*.db | tail -n +8 | xargs -r rm'` keeps the newest seven.

For a subscriptions-only backup, Settings → Export OPML.

## Restore

```sh
cd ~/yarrow
docker compose stop
docker compose run --rm --no-deps -v ~/yarrow-backups:/restore:ro \
  yarrow migrate -from /restore/storage-2026-10-14.db -to /data/storage.db -overwrite
docker compose up -d
```

`migrate` is reused for restores because it verifies the copy and never deletes the database it replaces (it is renamed to `storage.db.replaced-<timestamp>`).

## Moving to a new machine

Back up as above, copy the file to the new machine, clone the repository, and restore before the first `docker compose up`.
