#!/usr/bin/env bash
# Exercises a running API with an explicitly bootstrapped ADMIN. Requires curl/jq.
set -euo pipefail
: "${API_BASE:=http://localhost:8080}"
: "${ADMIN_EMAIL:?Set ADMIN_EMAIL to a bootstrapped administrator}"
: "${ADMIN_PASSWORD:?Set ADMIN_PASSWORD}"
: "${READER_EMAIL:?Set a fresh READER_EMAIL for this run}"
: "${READER_PASSWORD:?Set READER_PASSWORD (12+ characters)}"
command -v jq >/dev/null
API="$API_BASE/api/v1"

# Credentials are piped from jq and response tokens are retained only in shell variables.
reader=$(jq -n '{first_name:"Ada",last_name:"Lovelace",birth_year:1990,email:env.READER_EMAIL,password:env.READER_PASSWORD}' |
  curl --fail-with-body -sS -H 'Content-Type: application/json' --data-binary @- "$API/auth/register")
reader_id=$(jq -r '.data.id' <<<"$reader")
reader_login=$(jq -n '{email:env.READER_EMAIL,password:env.READER_PASSWORD}' |
  curl --fail-with-body -sS -H 'Content-Type: application/json' --data-binary @- "$API/auth/login")
reader_access=$(jq -r '.data.access_token' <<<"$reader_login")
reader_refresh=$(jq -r '.data.refresh_token' <<<"$reader_login")
admin_login=$(jq -n '{email:env.ADMIN_EMAIL,password:env.ADMIN_PASSWORD}' |
  curl --fail-with-body -sS -H 'Content-Type: application/json' --data-binary @- "$API/auth/login")
admin_access=$(jq -r '.data.access_token' <<<"$admin_login")
book=$(curl --fail-with-body -sS -H "Authorization: Bearer $admin_access" -H 'Content-Type: application/json' \
  -d '{"title":"Concurrency in Go","author":"Katherine Cox-Buday","publication_year":2017}' "$API/books")
book_id=$(jq -r '.data.id' <<<"$book")
curl --fail-with-body -sS -X PATCH -H "Authorization: Bearer $admin_access" -H 'Content-Type: application/json' \
  -d '{"title":"Concurrency in Go — Library Copy","volume":1}' "$API/books/$book_id" >/dev/null
curl --fail-with-body -sS -H "Authorization: Bearer $reader_access" "$API/books?page=1&page_size=20&sort=title&order=asc" >/dev/null
curl --fail-with-body -sS -H "Authorization: Bearer $reader_access" "$API/books/$book_id" >/dev/null
curl --fail-with-body -sS -X POST -H "Authorization: Bearer $reader_access" "$API/books/$book_id/favorite"
curl --fail-with-body -sS -X POST -H "Authorization: Bearer $reader_access" "$API/books/$book_id/like"
comment=$(curl --fail-with-body -sS -H "Authorization: Bearer $reader_access" -H 'Content-Type: application/json' \
  -d '{"content":"The concurrency examples are useful."}' "$API/books/$book_id/comments")
comment_id=$(jq -r '.data.id' <<<"$comment")
curl --fail-with-body -sS -H "Authorization: Bearer $reader_access" "$API/me/favorites" >/dev/null
rotated=$(jq -n --arg token "$reader_refresh" '{refresh_token:$token}' |
  curl --fail-with-body -sS -H 'Content-Type: application/json' --data-binary @- "$API/auth/refresh")
reader_access=$(jq -r '.data.access_token' <<<"$rotated")
reader_refresh=$(jq -r '.data.refresh_token' <<<"$rotated")
jq -n --arg token "$reader_refresh" '{refresh_token:$token}' |
  curl --fail-with-body -sS -H 'Content-Type: application/json' --data-binary @- "$API/auth/logout"
curl --fail-with-body -sS -H "Authorization: Bearer $admin_access" "$API/admin/readers?status=ACTIVE&sort=created_at&order=desc" >/dev/null
curl --fail-with-body -sS -X PATCH -H "Authorization: Bearer $admin_access" "$API/admin/readers/$reader_id/block"
blocked_status=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $reader_access" "$API/books")
test "$blocked_status" = 403
curl --fail-with-body -sS -X PATCH -H "Authorization: Bearer $admin_access" "$API/admin/readers/$reader_id/unblock"
curl --fail-with-body -sS -X DELETE -H "Authorization: Bearer $admin_access" "$API/books/$book_id/comments/$comment_id"
counts=$(curl --fail-with-body -sS -H "Authorization: Bearer $admin_access" "$API/books/$book_id")
jq -e '.data.likes_count == 1 and .data.comments_count == 0' <<<"$counts" >/dev/null
curl --fail-with-body -sS -X DELETE -H "Authorization: Bearer $admin_access" "$API/books/$book_id"
curl --fail-with-body -sS -X DELETE -H "Authorization: Bearer $admin_access" "$API/admin/readers/$reader_id"
printf '%s\n' 'PASS: reader and ADMIN curl flows, blocking and counters'
