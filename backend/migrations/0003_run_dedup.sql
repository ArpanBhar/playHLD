CREATE UNIQUE INDEX IF NOT EXISTS simulation_runs_exact_input_idx
ON simulation_runs (project_id, version_id, config_hash, engine_version, provenance)
WHERE provenance = 'server_native';
