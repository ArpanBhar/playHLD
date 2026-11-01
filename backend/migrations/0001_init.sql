CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (email = lower(email)),
  CHECK (length(email) BETWEEN 3 AND 254)
);

CREATE TABLE IF NOT EXISTS sessions (
  token_hash bytea PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  csrf_hash bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz
);
CREATE INDEX IF NOT EXISTS sessions_user_expires_idx ON sessions (user_id, expires_at);

CREATE TABLE IF NOT EXISTS designs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  graph_json jsonb NOT NULL CHECK (jsonb_typeof(graph_json) = 'object'),
  version bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS designs_owner_updated_idx ON designs (owner_id, updated_at DESC, id);

CREATE TABLE IF NOT EXISTS workspaces (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id uuid NOT NULL REFERENCES users(id),
  name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS workspaces_owner_idx ON workspaces (owner_id, created_at DESC);

CREATE TABLE IF NOT EXISTS workspace_members (
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('viewer', 'editor', 'admin', 'owner')),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX IF NOT EXISTS workspace_members_user_idx ON workspace_members (user_id, status);

CREATE TABLE IF NOT EXISTS projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  description text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS projects_workspace_idx ON projects (workspace_id, updated_at DESC, id);

CREATE TABLE IF NOT EXISTS architecture_branches (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
  draft_json jsonb NOT NULL CHECK (jsonb_typeof(draft_json) = 'object'),
  revision bigint NOT NULL DEFAULT 0 CHECK (revision >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, name)
);

CREATE TABLE IF NOT EXISTS architecture_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  branch_id uuid NOT NULL REFERENCES architecture_branches(id) ON DELETE CASCADE,
  parent_id uuid REFERENCES architecture_versions(id),
  branch_revision bigint NOT NULL CHECK (branch_revision >= 0),
  graph_json jsonb NOT NULL CHECK (jsonb_typeof(graph_json) = 'object'),
  graph_hash text NOT NULL,
  author_id uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (branch_id, branch_revision)
);
CREATE INDEX IF NOT EXISTS architecture_versions_project_idx ON architecture_versions (project_id, created_at DESC, id);

CREATE TABLE IF NOT EXISTS workload_profiles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  workload_json jsonb NOT NULL CHECK (jsonb_typeof(workload_json) = 'object'),
  author_id uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS simulation_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  version_id uuid NOT NULL REFERENCES architecture_versions(id),
  workload_id uuid REFERENCES workload_profiles(id),
  engine_version text NOT NULL,
  config_hash text NOT NULL,
  provenance text NOT NULL CHECK (provenance IN ('client_unverified', 'server_native')),
  summary_json jsonb NOT NULL CHECK (jsonb_typeof(summary_json) = 'object'),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS simulation_runs_project_idx ON simulation_runs (project_id, created_at DESC, id);

CREATE TABLE IF NOT EXISTS collaboration_operations (
  branch_id uuid NOT NULL REFERENCES architecture_branches(id) ON DELETE CASCADE,
  revision bigint NOT NULL CHECK (revision > 0),
  operation_id text NOT NULL,
  actor_id uuid NOT NULL REFERENCES users(id),
  payload_json jsonb NOT NULL CHECK (jsonb_typeof(payload_json) = 'object'),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (branch_id, revision),
  UNIQUE (branch_id, operation_id)
);

CREATE TABLE IF NOT EXISTS collaboration_snapshots (
  branch_id uuid NOT NULL REFERENCES architecture_branches(id) ON DELETE CASCADE,
  revision bigint NOT NULL CHECK (revision >= 0),
  graph_json jsonb NOT NULL CHECK (jsonb_typeof(graph_json) = 'object'),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (branch_id, revision)
);

CREATE TABLE IF NOT EXISTS challenges (
  id text NOT NULL,
  version integer NOT NULL CHECK (version > 0),
  content_json jsonb NOT NULL CHECK (jsonb_typeof(content_json) = 'object'),
  published_at timestamptz,
  PRIMARY KEY (id, version)
);

CREATE TABLE IF NOT EXISTS practice_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  challenge_id text NOT NULL,
  challenge_version integer NOT NULL,
  architecture_json jsonb NOT NULL CHECK (jsonb_typeof(architecture_json) = 'object'),
  result_json jsonb NOT NULL CHECK (jsonb_typeof(result_json) = 'object'),
  engine_version text NOT NULL,
  provenance text NOT NULL DEFAULT 'client_unverified' CHECK (provenance IN ('client_unverified', 'server_native')),
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (challenge_id, challenge_version) REFERENCES challenges(id, version)
);
CREATE INDEX IF NOT EXISTS practice_attempts_user_idx ON practice_attempts (user_id, challenge_id, created_at DESC);
