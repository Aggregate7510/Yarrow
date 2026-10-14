# Moving from Yarr

Yarrow uses Yarr's database schema **unchanged**. Everything Yarr stores comes across, and Yarrow's own preferences are extra keys in Yarr's existing `settings` table. The migration therefore is a verified copy, not a conversion.

## What is preserved

| Data | Preserved |
|---|---|
| Subscriptions (feed URL, site URL, custom titles) | Yes |
| Folders and which feeds are in them | Yes |
| Feed icons | Yes |
| Read / unread state of every article | Yes |
| Starred articles (shown as **Saved**) | Yes |
| Article history (titles, content, dates, media) | Yes, everything still in the database |
| Settings: theme, font, text size, sort order, refresh interval, last selected feed/folder and filter | Yes |
| HTTP caching state (ETag / Last-Modified) and last errors | Yes |

### What does not carry over

- **Yarr's interface layout preferences** (column widths) and **language**: stored, but unused by Yarrow's interface. Yarrow's UI is English only.
- **Login credentials** are not in the database. Set `YARR_AUTH` (or `YARR_AUTHFILE`) again; the same variables as Yarr are used.
- **Fever API clients** keep working with the same credentials.
- **PostgreSQL** Yarr databases cannot be copied with the tool below. Export OPML from Yarr and import it (read state is lost that way).
- Articles Yarr had already pruned (older than 90 days, beyond the newest 50 per feed) are gone in Yarr too.

### Saved vs. read

Like Yarr, an article is either *unread*, *read* or *starred*; Saved implies read. Removing an article from Saved makes it read, and "Mark as unread" is not offered for Saved articles. This keeps the database identical to Yarr's.

## Option A: migrate the database (recommended)

The `migrate` command opens the old database **read-only**, writes a backup copy with SQLite's `VACUUM INTO` (consistent even if a write-ahead log is present), copies it into place, upgrades older Yarr schemas, and compares folder, feed, article and read/unread/starred counts before declaring success. If any count differs, nothing is put in place.

1. **Stop Yarr** so it isn't writing during the copy.

   ```sh
   # whichever applies
   docker stop yarr
   sudo systemctl stop yarr
   ```

2. **Find the old database.** Common locations:
   - Docker: the file behind Yarr's `/data` volume, e.g. `docker volume inspect yarr_data` → `Mountpoint`, file `yarr.db`
   - Linux binary: `~/.config/yarr/storage.db`
   - macOS app: `~/Library/Application Support/yarr/storage.db`

3. **Make your own copy too** (belt and braces):

   ```sh
   cp /path/to/yarr.db ~/yarr-original-$(date +%F).db
   ```

4. **Run the migration into Yarrow's volume** (Yarrow must not be running):

   ```sh
   cd ~/yarrow
   docker compose stop
   docker compose run --rm --no-deps \
     -v /path/to/yarr-directory:/import:ro \
     yarrow migrate -from /import/yarr.db -to /data/storage.db -overwrite
   docker compose up -d
   ```

   `-overwrite` is only needed if Yarrow already created an empty database; the existing file is renamed to `storage.db.replaced-<timestamp>`, never deleted.

   Example output:

   ```
   source:  folders=6 feeds=10 items=1637 (unread=1632 read=3 starred=2) settings=2 schema=v8
   backup:  /data/yarr-backup-20261014-034323.db
   result:  folders=6 feeds=10 items=1637 (unread=1632 read=3 starred=2) settings=2 schema=v15
   migrated: /data/storage.db

   Migration complete and verified.
   ```

   Without Docker: `./yarrow migrate -from ~/.config/yarr/storage.db -to ./yarrow.db`.

5. Open Yarrow. Your folders, unread counts and Saved articles are there.

The migration is repeatable: run it again with `-overwrite` to start over from the untouched source.

### Tested upgrade paths

- Yarr **v2.4** database (schema v8) → upgraded to v15 inside the container, all counts and settings verified, source file checksum unchanged.
- Yarr **v2.9** reads a database written by Yarrow (rollback path) — verified by opening a migrated Yarrow database with an unmodified Yarr v2.9 build.

## Option B: OPML

In Yarr: menu → Subscriptions → Export. In Yarrow: Settings → Import OPML. Feeds and folders come across; read state, saved articles and history do not.

## Rolling back

Nothing in the procedure changes the original Yarr database, so rolling back is just:

1. `docker compose stop` (Yarrow).
2. Start Yarr again with its original database.

To take articles you saved or read in Yarrow back to Yarr v2.9 or later, point Yarr at a copy of Yarrow's database — the schema is the same. Back it up first:

```sh
docker compose exec yarrow yarrow backup -to /data/backups/for-yarr.db
docker cp yarrow:/data/backups/for-yarr.db ./for-yarr.db
```

Yarr releases older than v2.9 may not understand the newer schema version. Use the original database (or OPML) for those.
