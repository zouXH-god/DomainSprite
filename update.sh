#!/usr/bin/env bash
set -Eeuo pipefail

PROGRAM="$(basename "$0")"
DB_PATH="./database.db"
SERVICE_NAME="domainsprite.service"
MANAGE_SERVICE=1
START_SERVICE=1
BACKUP_PATH=""

usage() {
  cat <<'EOF'
DomainSprite SQLite migration helper

Usage:
  sudo ./update.sh [options]

Options:
  --db PATH          SQLite database path (default: ./database.db)
  --service NAME     systemd service name (default: domainsprite.service)
  --no-service       Do not stop or start a systemd service
  --no-start         Stop the service but do not start it after migration
  -h, --help         Show this help

The script creates a verified SQLite backup before changing the database. It
can be run repeatedly; databases that already contain domains.db_id are left
unchanged.
EOF
}

log() { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
die() { printf '[ERROR] %s\n' "$*" >&2; exit 1; }

while (($#)); do
  case "$1" in
    --db)
      (($# >= 2)) || die "--db requires a path"
      DB_PATH="$2"
      shift 2
      ;;
    --service)
      (($# >= 2)) || die "--service requires a name"
      SERVICE_NAME="$2"
      shift 2
      ;;
    --no-service)
      MANAGE_SERVICE=0
      shift
      ;;
    --no-start)
      START_SERVICE=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *) die "unknown option: $1" ;;
  esac
done

command -v sqlite3 >/dev/null 2>&1 || die "sqlite3 is required"
if ((MANAGE_SERVICE)); then
  command -v systemctl >/dev/null 2>&1 || die "systemctl is required (or use --no-service)"
  [[ ${EUID:-$(id -u)} -eq 0 ]] || die "run as root to manage systemd, or use --no-service"
fi

[[ -f "$DB_PATH" ]] || die "database not found: $DB_PATH"
DB_DIR="$(cd "$(dirname "$DB_PATH")" && pwd -P)"
DB_PATH="$DB_DIR/$(basename "$DB_PATH")"

on_error() {
  local line="$1"
  printf '[ERROR] migration failed at line %s\n' "$line" >&2
  if [[ -n "$BACKUP_PATH" && -f "$BACKUP_PATH" ]]; then
    printf '[ERROR] verified backup: %s\n' "$BACKUP_PATH" >&2
    printf '[ERROR] restore while the service is stopped with:\n' >&2
    printf '  cp -a -- %q %q\n' "$BACKUP_PATH" "$DB_PATH" >&2
  fi
}
trap 'on_error "$LINENO"' ERR

if ((MANAGE_SERVICE)); then
  log "stopping $SERVICE_NAME"
  systemctl stop "$SERVICE_NAME"
fi

integrity="$(sqlite3 -batch -noheader "$DB_PATH" 'PRAGMA integrity_check;')"
[[ "$integrity" == "ok" ]] || die "database integrity check failed: $integrity"

stamp="$(date '+%Y%m%d-%H%M%S')-$$"
BACKUP_PATH="$DB_PATH.bak.$stamp"
escaped_backup=${BACKUP_PATH//\'/\'\'}
log "creating SQLite backup: $BACKUP_PATH"
sqlite3 -batch "$DB_PATH" ".backup '$escaped_backup'"
chmod 0600 "$BACKUP_PATH"
backup_integrity="$(sqlite3 -batch -noheader "$BACKUP_PATH" 'PRAGMA integrity_check;')"
[[ "$backup_integrity" == "ok" ]] || die "backup integrity check failed: $backup_integrity"

has_table() {
  [[ "$(sqlite3 -batch -noheader "$DB_PATH" "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='$1';")" == "1" ]]
}

has_column() {
  [[ "$(sqlite3 -batch -noheader "$DB_PATH" "SELECT COUNT(*) FROM pragma_table_info('domains') WHERE name='$1';")" == "1" ]]
}

if ! has_table domains; then
  log "domains table does not exist; application migration will create it"
elif has_column db_id; then
  pk="$(sqlite3 -batch -noheader "$DB_PATH" "SELECT pk FROM pragma_table_info('domains') WHERE name='db_id';")"
  [[ "$pk" == "1" ]] || die "domains.db_id exists but is not the primary key; manual inspection required"
  log "domains table is already migrated"
else
  log "legacy domains table detected; rebuilding it with db_id primary key"

  account_lookup="''"
  account_id_lookup="0"
  if has_table dns_accounts; then
    account_lookup="COALESCE((SELECT a.name FROM dns_accounts a WHERE lower(a.provider_type)=lower(d.dns_from) AND a.enabled=1 ORDER BY a.id LIMIT 1), '')"
    account_id_lookup="COALESCE((SELECT a.id FROM dns_accounts a WHERE lower(a.provider_type)=lower(d.dns_from) AND a.enabled=1 ORDER BY a.id LIMIT 1), 0)"
  fi
  if has_column account_name; then
    account_expr="COALESCE(NULLIF(TRIM(d.account_name), ''), $account_lookup)"
  else
    account_expr="$account_lookup"
  fi
  if has_column dns_account_id; then
    account_id_expr="COALESCE(NULLIF(d.dns_account_id, 0), $account_id_lookup)"
  else
    account_id_expr="$account_id_lookup"
  fi

  column_expr() {
    local column="$1" fallback="$2"
    if has_column "$column"; then
      printf 'd.%s' "$column"
    else
      printf '%s' "$fallback"
    fi
  }

  id_expr="$(column_expr id "''")"
  domain_name_expr="$(column_expr domain_name "''")"
  group_id_expr="$(column_expr group_id "NULL")"
  group_name_expr="$(column_expr group_name "NULL")"
  status_expr="$(column_expr status "NULL")"
  type_expr="$(column_expr type "NULL")"
  certificate_id_expr="$(column_expr certificate_id "0")"
  create_time_expr="$(column_expr create_time "NULL")"
  update_time_expr="$(column_expr update_time "NULL")"
  dns_from_expr="$(column_expr dns_from "''")"

  expected_rows="$(sqlite3 -batch -noheader "$DB_PATH" "SELECT COUNT(*) FROM (SELECT 1 FROM domains d GROUP BY $account_expr, $dns_from_expr, $id_expr);")"

  sqlite3 -batch "$DB_PATH" <<SQL
.bail on
PRAGMA foreign_keys=OFF;
BEGIN IMMEDIATE;
CREATE TABLE domains_migrating (
  db_id INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL,
  domain_name TEXT NOT NULL,
  group_id TEXT,
  group_name TEXT,
  status TEXT,
  type TEXT,
  certificate_id INTEGER DEFAULT 0,
  create_time datetime,
  update_time datetime,
  dns_from TEXT NOT NULL,
  account_name TEXT NOT NULL DEFAULT '',
  dns_account_id INTEGER DEFAULT 0
);
WITH source AS (
  SELECT
    d.rowid AS source_rowid,
    $id_expr AS id,
    $domain_name_expr AS domain_name,
    $group_id_expr AS group_id,
    $group_name_expr AS group_name,
    $status_expr AS status,
    $type_expr AS type,
    $certificate_id_expr AS certificate_id,
    $create_time_expr AS create_time,
    $update_time_expr AS update_time,
    $dns_from_expr AS dns_from,
    $account_expr AS account_name,
    $account_id_expr AS dns_account_id
  FROM domains d
), ranked AS (
  SELECT *, ROW_NUMBER() OVER (
    PARTITION BY account_name, dns_from, id
    ORDER BY source_rowid DESC
  ) AS duplicate_rank
  FROM source
)
INSERT INTO domains_migrating (
  id, domain_name, group_id, group_name, status, type, certificate_id,
  create_time, update_time, dns_from, account_name, dns_account_id
)
SELECT
  id, domain_name, group_id, group_name, status, type, certificate_id,
  create_time, update_time, dns_from, account_name, dns_account_id
FROM ranked
WHERE duplicate_rank=1;
DROP TABLE domains;
ALTER TABLE domains_migrating RENAME TO domains;
CREATE UNIQUE INDEX idx_domain_identity ON domains(account_name, dns_from, id);
CREATE INDEX idx_domains_dns_account_id ON domains(dns_account_id);
COMMIT;
PRAGMA foreign_keys=ON;
SQL

  actual_rows="$(sqlite3 -batch -noheader "$DB_PATH" 'SELECT COUNT(*) FROM domains;')"
  [[ "$actual_rows" == "$expected_rows" ]] || die "row count mismatch: expected $expected_rows, got $actual_rows"
  pk="$(sqlite3 -batch -noheader "$DB_PATH" "SELECT pk FROM pragma_table_info('domains') WHERE name='db_id';")"
  [[ "$pk" == "1" ]] || die "db_id primary key verification failed"
  log "domains migration complete: $actual_rows rows retained"
fi

integrity="$(sqlite3 -batch -noheader "$DB_PATH" 'PRAGMA integrity_check;')"
[[ "$integrity" == "ok" ]] || die "post-migration integrity check failed: $integrity"

log "database migration completed successfully"
log "backup retained at: $BACKUP_PATH"

if ((MANAGE_SERVICE && START_SERVICE)); then
  log "starting $SERVICE_NAME"
  systemctl start "$SERVICE_NAME"
  sleep 2
  if ! systemctl is-active --quiet "$SERVICE_NAME"; then
    systemctl status "$SERVICE_NAME" --no-pager || true
    die "$SERVICE_NAME failed to start; database backup was retained"
  fi
  log "$SERVICE_NAME is active"
fi
