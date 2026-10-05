# Complete curl flows

The executable [scripts/smoke.sh](../scripts/smoke.sh) contains the complete flow
below with strict status checking and no token/password output. It creates and
then removes a book and soft-deletes its test reader. Use a fresh reader email
because deleted emails remain reserved. Requirements: Bash, curl, jq; an existing
ADMIN provisioned with the bootstrap command in the README.

```bash
export API_BASE=http://localhost:18080 # use 8080 for default configuration
export ADMIN_EMAIL='admin@example.com'
export READER_EMAIL="reader-$(date +%s)@example.com"
read -rsp 'Admin password: ' ADMIN_PASSWORD; echo
read -rsp 'Reader password (12+ characters): ' READER_PASSWORD; echo
export ADMIN_PASSWORD READER_PASSWORD
./scripts/smoke.sh
unset ADMIN_PASSWORD READER_PASSWORD
```

The script performs these concrete requests:

| Reader flow | Administrator flow |
| --- | --- |
| POST `/auth/register` | POST `/auth/login` |
| POST `/auth/login` | POST `/books` |
| GET `/books` | PATCH `/books/:id` |
| GET `/books/:id` | GET `/admin/readers` |
| POST `/books/:id/favorite` | PATCH `/admin/readers/:id/block` |
| POST `/books/:id/like` | PATCH `/admin/readers/:id/unblock` |
| POST `/books/:id/comments` | DELETE `/books/:id/comments/:commentId` |
| GET `/me/favorites` | DELETE `/books/:id` |
| POST `/auth/refresh` | DELETE `/admin/readers/:id` (test cleanup) |
| POST `/auth/logout` | |

To use the calls individually, retain the IDs and access token from JSON `data`
fields. Examples (shell variables refer to values from the previous response):

```bash
API="$API_BASE/api/v1"

# Register; public JSON has no role field.
jq -n '{first_name:"Ada",last_name:"Lovelace",birth_year:1990,
        email:env.READER_EMAIL,password:env.READER_PASSWORD}' |
  curl -sS --fail-with-body -H 'Content-Type: application/json' \
    --data-binary @- "$API/auth/register"

# Log in; store credentials privately instead of printing them.
reader_login=$(jq -n '{email:env.READER_EMAIL,password:env.READER_PASSWORD}' |
  curl -sS --fail-with-body -H 'Content-Type: application/json' \
    --data-binary @- "$API/auth/login")
reader_access=$(jq -r '.data.access_token' <<<"$reader_login")
reader_refresh=$(jq -r '.data.refresh_token' <<<"$reader_login")

curl -sS --fail-with-body -H "Authorization: Bearer $reader_access" \
  "$API/books?page=1&page_size=20&title=clean&sort=title&order=asc"
curl -sS --fail-with-body -H "Authorization: Bearer $reader_access" \
  "$API/books/$book_id"
curl -sS --fail-with-body -X POST -H "Authorization: Bearer $reader_access" \
  "$API/books/$book_id/favorite"
curl -sS --fail-with-body -X POST -H "Authorization: Bearer $reader_access" \
  "$API/books/$book_id/like"
curl -sS --fail-with-body -H "Authorization: Bearer $reader_access" \
  -H 'Content-Type: application/json' -d '{"content":"A useful book."}' \
  "$API/books/$book_id/comments"
curl -sS --fail-with-body -H "Authorization: Bearer $reader_access" "$API/me/favorites"

rotated=$(jq -n --arg token "$reader_refresh" '{refresh_token:$token}' |
  curl -sS --fail-with-body -H 'Content-Type: application/json' \
    --data-binary @- "$API/auth/refresh")
reader_refresh=$(jq -r '.data.refresh_token' <<<"$rotated")
jq -n --arg token "$reader_refresh" '{refresh_token:$token}' |
  curl -sS --fail-with-body -H 'Content-Type: application/json' \
    --data-binary @- "$API/auth/logout"

# Admin login and book management.
admin_login=$(jq -n '{email:env.ADMIN_EMAIL,password:env.ADMIN_PASSWORD}' |
  curl -sS --fail-with-body -H 'Content-Type: application/json' \
    --data-binary @- "$API/auth/login")
admin_access=$(jq -r '.data.access_token' <<<"$admin_login")
book=$(curl -sS --fail-with-body -H "Authorization: Bearer $admin_access" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Clean Code","author":"Robert Martin","publication_year":2008}' \
  "$API/books")
book_id=$(jq -r '.data.id' <<<"$book")
curl -sS --fail-with-body -X PATCH -H "Authorization: Bearer $admin_access" \
  -H 'Content-Type: application/json' -d '{"title":"Clean Code Revised"}' \
  "$API/books/$book_id"
curl -sS --fail-with-body -H "Authorization: Bearer $admin_access" \
  "$API/admin/readers?page=1&page_size=20&status=ACTIVE&sort=created_at&order=desc"
curl -sS --fail-with-body -X PATCH -H "Authorization: Bearer $admin_access" \
  "$API/admin/readers/$reader_id/block"
curl -sS --fail-with-body -X PATCH -H "Authorization: Bearer $admin_access" \
  "$API/admin/readers/$reader_id/unblock"
curl -sS --fail-with-body -X DELETE -H "Authorization: Bearer $admin_access" \
  "$API/books/$book_id/comments/$comment_id"
curl -sS --fail-with-body -X DELETE -H "Authorization: Bearer $admin_access" \
  "$API/books/$book_id"
```

Use the script for a correctly ordered, self-contained run: the administrator
creates the book before the reader interacts with it. After blocking/unblocking or
password changes, old access tokens remain invalid; log in again. After logout,
refresh is revoked but an already issued access token lasts until expiry.
