CREATE TABLE users (
 id uuid PRIMARY KEY,
 first_name varchar(100) NOT NULL CHECK (length(btrim(first_name)) BETWEEN 1 AND 100),
 last_name varchar(100) NOT NULL CHECK (length(btrim(last_name)) BETWEEN 1 AND 100),
 middle_name varchar(100),
 birth_year integer NOT NULL CHECK (birth_year BETWEEN 1900 AND 2200),
 email varchar(254) NOT NULL UNIQUE CHECK (email = lower(btrim(email)) AND position('@' in email) > 1),
 password_hash text NOT NULL,
 role varchar(10) NOT NULL CHECK (role IN ('ADMIN','READER')),
 status varchar(10) NOT NULL CHECK (status IN ('ACTIVE','BLOCKED')),
 auth_version bigint NOT NULL DEFAULT 1 CHECK (auth_version > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 deleted_at timestamptz
);
CREATE INDEX users_readers_created ON users (status, created_at, id) WHERE role = 'READER' AND deleted_at IS NULL;
CREATE TABLE books (
 id uuid PRIMARY KEY,
 title varchar(300) NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 300),
 author varchar(200) NOT NULL CHECK (length(btrim(author)) BETWEEN 1 AND 200),
 publication_year integer NOT NULL CHECK (publication_year BETWEEN 1 AND 2200),
 volume integer CHECK (volume > 0),
 likes_count bigint NOT NULL DEFAULT 0 CHECK (likes_count >= 0),
 comments_count bigint NOT NULL DEFAULT 0 CHECK (comments_count >= 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX books_created ON books (created_at, id);
CREATE INDEX books_year ON books (publication_year, id);
CREATE TABLE reader_favorites (
 reader_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (reader_id, book_id)
);
CREATE INDEX reader_favorites_book ON reader_favorites(book_id);
CREATE INDEX reader_favorites_recent ON reader_favorites(reader_id, created_at, book_id);
CREATE TABLE book_likes (
 reader_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (reader_id, book_id)
);
CREATE INDEX book_likes_book ON book_likes(book_id);
CREATE TABLE comments (
 id uuid PRIMARY KEY,
 book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
 reader_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 content varchar(4000) NOT NULL CHECK (length(btrim(content)) BETWEEN 1 AND 4000),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX comments_book_created ON comments(book_id, created_at, id);
CREATE INDEX comments_reader ON comments(reader_id);
CREATE TABLE refresh_tokens (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 family_id uuid NOT NULL,
 token_hash char(64) NOT NULL UNIQUE,
 expires_at timestamptz NOT NULL,
 consumed_at timestamptz,
 revoked_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK (expires_at > created_at)
);
CREATE INDEX refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX refresh_tokens_family ON refresh_tokens(family_id);
CREATE INDEX refresh_tokens_expiry ON refresh_tokens(expires_at);
