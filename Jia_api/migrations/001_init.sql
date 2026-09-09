PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  display_name TEXT NOT NULL,
  avatar_url TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TEXT NOT NULL,
  used_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens(user_id);

CREATE TABLE IF NOT EXISTS families (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  surname TEXT NOT NULL DEFAULT '',
  origin TEXT NOT NULL DEFAULT '',
  migration_history TEXT NOT NULL DEFAULT '',
  creed TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS family_members (
  id TEXT PRIMARY KEY,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('owner','editor','viewer')),
  status TEXT NOT NULL DEFAULT 'active',
  joined_at TEXT NOT NULL,
  UNIQUE(family_id, user_id)
);

CREATE TABLE IF NOT EXISTS invites (
  id TEXT PRIMARY KEY,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  email TEXT NOT NULL,
  token TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL CHECK (role IN ('editor','viewer')),
  expires_at TEXT NOT NULL,
  accepted_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS genealogies (
  id TEXT PRIMARY KEY,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS persons (
  id TEXT PRIMARY KEY,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  gender TEXT NOT NULL DEFAULT 'unknown' CHECK (gender IN ('male','female','unknown')),
  birth_date TEXT NOT NULL DEFAULT '',
  death_date TEXT NOT NULL DEFAULT '',
  birthplace TEXT NOT NULL DEFAULT '',
  occupation TEXT NOT NULL DEFAULT '',
  biography TEXT NOT NULL DEFAULT '',
  claimed_by TEXT REFERENCES users(id),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS genealogy_persons (
  genealogy_id TEXT NOT NULL REFERENCES genealogies(id) ON DELETE CASCADE,
  person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  PRIMARY KEY (genealogy_id, person_id)
);

CREATE TABLE IF NOT EXISTS person_relations (
  id TEXT PRIMARY KEY,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  from_person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  to_person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  relation_type TEXT NOT NULL,
  custom_name TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  inverse_id TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL,
  UNIQUE(family_id, from_person_id, to_person_id, relation_type, custom_name)
);

CREATE TABLE IF NOT EXISTS books (
  id TEXT PRIMARY KEY,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  root_person_id TEXT REFERENCES persons(id),
  content_json TEXT NOT NULL DEFAULT '{"type":"doc","content":[{"type":"paragraph"}]}',
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS book_collaborators (
  book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (book_id, user_id)
);

CREATE TABLE IF NOT EXISTS photos (
  id TEXT PRIMARY KEY,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  path TEXT NOT NULL,
  original_name TEXT NOT NULL,
  caption TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT 'other',
  taken_at TEXT NOT NULL DEFAULT '',
  location TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS photo_persons (
  photo_id TEXT NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
  person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  PRIMARY KEY (photo_id, person_id)
);

CREATE TABLE IF NOT EXISTS activities (
  id TEXT PRIMARY KEY,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  actor_id TEXT NOT NULL REFERENCES users(id),
  type TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  summary TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_family_members_user ON family_members(user_id);
CREATE INDEX IF NOT EXISTS idx_genealogies_family ON genealogies(family_id);
CREATE INDEX IF NOT EXISTS idx_persons_family ON persons(family_id);
CREATE INDEX IF NOT EXISTS idx_relations_family ON person_relations(family_id);
CREATE INDEX IF NOT EXISTS idx_books_family ON books(family_id);
CREATE INDEX IF NOT EXISTS idx_activities_family ON activities(family_id, created_at DESC);

-- 人物可被多个家族关联（主家族在 persons.family_id，此处记录额外关联的家族；
-- 各家族的关系图只使用本家族的 person_relations，人物关系互不共用）。
CREATE TABLE IF NOT EXISTS person_families (
  person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  PRIMARY KEY (person_id, family_id)
);
CREATE INDEX IF NOT EXISTS idx_person_families_family ON person_families(family_id);
