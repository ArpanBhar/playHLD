ALTER TABLE simulation_runs ADD COLUMN IF NOT EXISTS workload_json jsonb CHECK (workload_json IS NULL OR jsonb_typeof(workload_json) = 'object');
ALTER TABLE simulation_runs ADD COLUMN IF NOT EXISTS result_json jsonb CHECK (result_json IS NULL OR jsonb_typeof(result_json) = 'object');
