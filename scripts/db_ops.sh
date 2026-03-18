#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_DB_PATH="logs/coraza/mamotama.db"
DEFAULT_BACKUP_DIR="data/logs/coraza/backups"

usage() {
  cat <<'USAGE'
Usage:
  ./scripts/db_ops.sh info
  ./scripts/db_ops.sh backup [backup_file]
  ./scripts/db_ops.sh restore <backup_file>
  ./scripts/db_ops.sh vacuum

Environment:
  WAF_DB_PATH  Defaults to logs/coraza/mamotama.db

Notes:
  - Relative WAF_DB_PATH values like logs/... are mapped under ./data/.
  - backup uses sqlite3 .backup when available; otherwise it copies DB(+wal/+shm).
  - restore should be executed while coraza is stopped to avoid write races.
USAGE
}

abs_path() {
  local p="$1"
  if [[ "${p}" = /* ]]; then
    printf '%s\n' "${p}"
    return
  fi
  printf '%s/%s\n' "${ROOT_DIR}" "${p}"
}

resolve_db_file() {
  local raw="${WAF_DB_PATH:-${DEFAULT_DB_PATH}}"
  raw="${raw#"${raw%%[![:space:]]*}"}"
  raw="${raw%"${raw##*[![:space:]]}"}"
  if [[ -z "${raw}" ]]; then
    raw="${DEFAULT_DB_PATH}"
  fi
  if [[ "${raw}" = /* ]]; then
    printf '%s\n' "${raw}"
    return
  fi

  case "${raw}" in
    logs/*|conf/*|rules/*)
      printf '%s/data/%s\n' "${ROOT_DIR}" "${raw}"
      ;;
    data/*)
      printf '%s/%s\n' "${ROOT_DIR}" "${raw}"
      ;;
    *)
      printf '%s/%s\n' "${ROOT_DIR}" "${raw}"
      ;;
  esac
}

do_info() {
  local db_file="$1"
  echo "db_file=${db_file}"
  if [[ -f "${db_file}" ]]; then
    local size
    size="$(wc -c <"${db_file}" | tr -d ' ')"
    echo "exists=true"
    echo "size_bytes=${size}"
  else
    echo "exists=false"
  fi
}

do_backup() {
  local db_file="$1"
  local target="${2:-}"
  local ts
  ts="$(date +%Y%m%d-%H%M%S)"

  if [[ -z "${target}" ]]; then
    target="$(abs_path "${DEFAULT_BACKUP_DIR}/mamotama-${ts}.db")"
  else
    target="$(abs_path "${target}")"
  fi
  mkdir -p "$(dirname "${target}")"

  if [[ ! -f "${db_file}" ]]; then
    echo "[db_ops] db file not found: ${db_file}" >&2
    exit 1
  fi

  if command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 "${db_file}" ".timeout 5000" ".backup '${target}'"
    echo "[db_ops] sqlite backup completed: ${target}"
    return
  fi

  cp -f "${db_file}" "${target}"
  if [[ -f "${db_file}-wal" ]]; then
    cp -f "${db_file}-wal" "${target}-wal"
  fi
  if [[ -f "${db_file}-shm" ]]; then
    cp -f "${db_file}-shm" "${target}-shm"
  fi
  echo "[db_ops] sqlite3 not found; copied db files: ${target}"
}

do_restore() {
  local db_file="$1"
  local backup_file="$2"

  backup_file="$(abs_path "${backup_file}")"
  if [[ ! -f "${backup_file}" ]]; then
    echo "[db_ops] backup file not found: ${backup_file}" >&2
    exit 1
  fi

  mkdir -p "$(dirname "${db_file}")"
  cp -f "${backup_file}" "${db_file}"

  if [[ -f "${backup_file}-wal" ]]; then
    cp -f "${backup_file}-wal" "${db_file}-wal"
  else
    rm -f "${db_file}-wal"
  fi
  if [[ -f "${backup_file}-shm" ]]; then
    cp -f "${backup_file}-shm" "${db_file}-shm"
  else
    rm -f "${db_file}-shm"
  fi

  echo "[db_ops] restore completed: ${db_file}"
}

do_vacuum() {
  local db_file="$1"
  if [[ ! -f "${db_file}" ]]; then
    echo "[db_ops] db file not found: ${db_file}" >&2
    exit 1
  fi
  if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "[db_ops] sqlite3 is required for vacuum" >&2
    exit 1
  fi
  sqlite3 "${db_file}" "PRAGMA wal_checkpoint(TRUNCATE); VACUUM;"
  echo "[db_ops] vacuum completed: ${db_file}"
}

main() {
  if [[ "${1:-}" == "" || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    return 0
  fi

  local db_file
  db_file="$(resolve_db_file)"

  local cmd="$1"
  shift || true

  case "${cmd}" in
    info)
      do_info "${db_file}"
      ;;
    backup)
      do_backup "${db_file}" "${1:-}"
      ;;
    restore)
      if [[ "${1:-}" == "" ]]; then
        echo "[db_ops] restore requires backup file path" >&2
        usage >&2
        exit 1
      fi
      do_restore "${db_file}" "$1"
      ;;
    vacuum)
      do_vacuum "${db_file}"
      ;;
    *)
      echo "[db_ops] unknown command: ${cmd}" >&2
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
