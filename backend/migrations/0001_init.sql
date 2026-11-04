CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6)))),
  email TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK (email = lower(email)),
  CHECK (length(email) BETWEEN 3 AND 254)
);

CREATE TABLE IF NOT EXISTS sessions (
  token_hash BLOB PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  csrf_hash BLOB NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME NOT NULL,
  revoked_at DATETIME
);
CREATE INDEX IF NOT EXISTS sessions_user_expires_idx ON sessions (user_id, expires_at);

CREATE TABLE IF NOT EXISTS designs (
  id TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6)))),
  owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  graph_json TEXT NOT NULL CHECK (json_valid(graph_json) AND json_type(graph_json) = 'object'),
  version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS designs_owner_updated_idx ON designs (owner_id, updated_at DESC, id);

CREATE TABLE IF NOT EXISTS workspaces (
  id TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6)))),
  owner_id TEXT NOT NULL REFERENCES users(id),
  name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS workspaces_owner_idx ON workspaces (owner_id, created_at DESC);

CREATE TABLE IF NOT EXISTS workspace_members (
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('viewer', 'editor', 'admin', 'owner')),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
  joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX IF NOT EXISTS workspace_members_user_idx ON workspace_members (user_id, status);

CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6)))),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  description TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS projects_workspace_idx ON projects (workspace_id, updated_at DESC, id);

CREATE TABLE IF NOT EXISTS architecture_branches (
  id TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6)))),
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
  draft_json TEXT NOT NULL CHECK (json_valid(draft_json) AND json_type(draft_json) = 'object'),
  revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (project_id, name)
);

CREATE TABLE IF NOT EXISTS architecture_versions (
  id TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6)))),
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  branch_id TEXT NOT NULL REFERENCES architecture_branches(id) ON DELETE CASCADE,
  parent_id TEXT REFERENCES architecture_versions(id),
  branch_revision INTEGER NOT NULL CHECK (branch_revision >= 0),
  graph_json TEXT NOT NULL CHECK (json_valid(graph_json) AND json_type(graph_json) = 'object'),
  graph_hash TEXT NOT NULL,
  author_id TEXT NOT NULL REFERENCES users(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (branch_id, branch_revision)
);
CREATE INDEX IF NOT EXISTS architecture_versions_project_idx ON architecture_versions (project_id, created_at DESC, id);

CREATE TABLE IF NOT EXISTS workload_profiles (
  id TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6)))),
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  workload_json TEXT NOT NULL CHECK (json_valid(workload_json) AND json_type(workload_json) = 'object'),
  author_id TEXT NOT NULL REFERENCES users(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS simulation_runs (
  id TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6)))),
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  version_id TEXT NOT NULL REFERENCES architecture_versions(id),
  workload_id TEXT REFERENCES workload_profiles(id),
  engine_version TEXT NOT NULL,
  config_hash TEXT NOT NULL,
  provenance TEXT NOT NULL CHECK (provenance IN ('client_unverified', 'server_native')),
  summary_json TEXT NOT NULL CHECK (json_valid(summary_json) AND json_type(summary_json) = 'object'),
  workload_json TEXT CHECK (workload_json IS NULL OR (json_valid(workload_json) AND json_type(workload_json) = 'object')),
  result_json TEXT CHECK (result_json IS NULL OR (json_valid(result_json) AND json_type(result_json) = 'object')),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS simulation_runs_project_idx ON simulation_runs (project_id, created_at DESC, id);

CREATE TABLE IF NOT EXISTS collaboration_operations (
  branch_id TEXT NOT NULL REFERENCES architecture_branches(id) ON DELETE CASCADE,
  revision INTEGER NOT NULL CHECK (revision > 0),
  operation_id TEXT NOT NULL,
  actor_id TEXT NOT NULL REFERENCES users(id),
  payload_json TEXT NOT NULL CHECK (json_valid(payload_json) AND json_type(payload_json) = 'object'),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (branch_id, revision),
  UNIQUE (branch_id, operation_id)
);

CREATE TABLE IF NOT EXISTS collaboration_snapshots (
  branch_id TEXT NOT NULL REFERENCES architecture_branches(id) ON DELETE CASCADE,
  revision INTEGER NOT NULL CHECK (revision >= 0),
  graph_json TEXT NOT NULL CHECK (json_valid(graph_json) AND json_type(graph_json) = 'object'),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (branch_id, revision)
);

CREATE TABLE IF NOT EXISTS challenges (
  id TEXT NOT NULL,
  version INTEGER NOT NULL CHECK (version > 0),
  content_json TEXT NOT NULL CHECK (json_valid(content_json) AND json_type(content_json) = 'object'),
  published_at DATETIME,
  PRIMARY KEY (id, version)
);

CREATE TABLE IF NOT EXISTS practice_attempts (
  id TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6)))),
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  challenge_id TEXT NOT NULL,
  challenge_version INTEGER NOT NULL,
  architecture_json TEXT NOT NULL CHECK (json_valid(architecture_json) AND json_type(architecture_json) = 'object'),
  result_json TEXT NOT NULL CHECK (json_valid(result_json) AND json_type(result_json) = 'object'),
  engine_version TEXT NOT NULL,
  provenance TEXT NOT NULL DEFAULT 'client_unverified' CHECK (provenance IN ('client_unverified', 'server_native')),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (challenge_id, challenge_version) REFERENCES challenges(id, version)
);
CREATE INDEX IF NOT EXISTS practice_attempts_user_idx ON practice_attempts (user_id, challenge_id, created_at DESC);
