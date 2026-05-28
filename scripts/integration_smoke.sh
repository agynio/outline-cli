#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${OUTLINE_BASE_URL:-}" || -z "${OUTLINE_API_KEY:-}" ]]; then
  cat >&2 <<'USAGE'
Required environment variables:
  OUTLINE_BASE_URL   Outline instance URL, e.g. https://wiki.example.com
  OUTLINE_API_KEY    Outline API key

Optional environment variables:
  OUTLINE_BIN        outline binary to run (default: go run ./cmd/outline --)
  OUTLINE_HOME       HOME directory for isolated CLI config (default: temporary dir)
  OUTLINE_COLLECTION Collection name to resolve (default: Test)
USAGE
  exit 2
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
created_outline_home=""
if [[ -z "${OUTLINE_HOME:-}" ]]; then
  OUTLINE_HOME="$(mktemp -d)"
  created_outline_home="${OUTLINE_HOME}"
fi
OUTLINE_COLLECTION="${OUTLINE_COLLECTION:-Test}"
if [[ -n "${OUTLINE_BIN:-}" ]]; then
  OUTLINE_CMD=("${OUTLINE_BIN}")
else
  OUTLINE_CMD=(go run ./cmd/outline --)
fi

TMP_DIR="$(mktemp -d)"
RESULTS_FILE="${TMP_DIR}/results.tsv"
DOC_TEXT_FILE="${TMP_DIR}/document.md"
printf '# Temporary integration document\n\nCreated by outline-cli integration smoke runner.\n' >"${DOC_TEXT_FILE}"

cleanup() {
  rm -rf "${TMP_DIR}"
  if [[ -n "${created_outline_home}" ]]; then
    rm -rf "${created_outline_home}"
  fi
}
trap cleanup EXIT

run_outline() {
  HOME="${OUTLINE_HOME}" "${OUTLINE_CMD[@]}" "$@"
}

record() {
  local method="$1"
  local outcome="$2"
  printf '%s\t%s\n' "${method}" "${outcome}" >>"${RESULTS_FILE}"
}

run_method() {
  local method="$1"
  shift
  local output
  set +e
  output="$(run_outline "$@" 2>&1)"
  local status=$?
  set -e
  if [[ ${status} -eq 0 ]]; then
    record "${method}" "ok"
  else
    output="$(printf '%s' "${output}" | tr '\n\t' '  ' | cut -c1-160)"
    record "${method}" "fail: ${output}"
  fi
}

extract_id_by_name() {
  local json_file="$1"
  local name="$2"
  node -e '
const fs = require("fs");
const input = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
const name = process.argv[2];
const rows = Array.isArray(input.data) ? input.data : Array.isArray(input) ? input : [];
const found = rows.find((row) => row && row.name === name);
if (!found || !found.id) process.exit(1);
process.stdout.write(found.id);
' "${json_file}" "${name}"
}

extract_data_id() {
  local json_file="$1"
  node -e '
const fs = require("fs");
const input = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
const data = input.data || input;
if (!data || !data.id) process.exit(1);
process.stdout.write(data.id);
' "${json_file}"
}

cd "${ROOT_DIR}"
: >"${RESULTS_FILE}"

run_method "auth.login" auth login --base-url "${OUTLINE_BASE_URL}" --api-key "${OUTLINE_API_KEY}"
run_method "auth.info" auth info --output json
run_method "auth.config" auth config --output json

collections_json="${TMP_DIR}/collections.json"
if run_outline collections list --output json >"${collections_json}"; then
  record "collections.list" "ok"
else
  record "collections.list" "fail: unable to list collections"
fi

collection_id=""
if collection_id="$(extract_id_by_name "${collections_json}" "${OUTLINE_COLLECTION}" 2>/dev/null)"; then
  record "collections.resolve(${OUTLINE_COLLECTION})" "ok: ${collection_id}"
else
  record "collections.resolve(${OUTLINE_COLLECTION})" "fail: collection not found"
  printf 'METHOD\tOUTCOME\n'
  cat "${RESULTS_FILE}"
  exit 1
fi

run_method "collections.info" collections info --id "${collection_id}" --output json
run_method "collections.documents" collections documents --id "${collection_id}" --output json

created_json="${TMP_DIR}/created.json"
if run_outline documents create --collection-id "${collection_id}" --title "outline-cli smoke $(date -u +%Y%m%dT%H%M%SZ)" --file "${DOC_TEXT_FILE}" --output json >"${created_json}"; then
  record "documents.create" "ok"
else
  record "documents.create" "fail: unable to create temp document"
  printf 'METHOD\tOUTCOME\n'
  cat "${RESULTS_FILE}"
  exit 1
fi

document_id=""
if document_id="$(extract_data_id "${created_json}" 2>/dev/null)"; then
  record "documents.resolve(temp)" "ok: ${document_id}"
else
  record "documents.resolve(temp)" "fail: created document id missing"
  printf 'METHOD\tOUTCOME\n'
  cat "${RESULTS_FILE}"
  exit 1
fi

run_method "documents.info" documents info --id "${document_id}" --output json
run_method "documents.list" documents list --collection-id "${collection_id}" --output json
run_method "documents.search" documents search "outline-cli smoke" --collection-id "${collection_id}" --output json
run_method "documents.users" documents users --id "${document_id}" --output json
run_method "documents.documents" documents documents --id "${document_id}" --output json
run_method "comments.list" comments list --document-id "${document_id}" --output json
run_method "views.list" views list --document-id "${document_id}" --output json
run_method "views.create" views create --document-id "${document_id}" --output json
run_method "shares.list" shares list --document-id "${document_id}" --output json
run_method "stars.list" stars list --output json
run_method "groups.list" groups list --output json
run_method "events.list" events list --document-id "${document_id}" --output json

run_method "documents.delete" documents delete --id "${document_id}" --yes --output json

printf 'METHOD\tOUTCOME\n'
cat "${RESULTS_FILE}"
