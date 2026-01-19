-- prepare queue schema

CREATE TABLE IF NOT EXISTS prepare_jobs (
  job_id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  prepare_kind TEXT NOT NULL,
  image_id TEXT NOT NULL,
  plan_only INTEGER NOT NULL DEFAULT 0,
  snapshot_mode TEXT NOT NULL DEFAULT 'always',
  prepare_args_normalized TEXT,
  request_json TEXT,
  created_at TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT,
  result_json TEXT,
  error_json TEXT
);
CREATE INDEX IF NOT EXISTS idx_prepare_jobs_status ON prepare_jobs(status);

CREATE TABLE IF NOT EXISTS prepare_tasks (
  job_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  planner_kind TEXT,
  input_kind TEXT,
  input_id TEXT,
  task_hash TEXT,
  output_state_id TEXT,
  cached INTEGER,
  instance_mode TEXT,
  started_at TEXT,
  finished_at TEXT,
  error_json TEXT,
  PRIMARY KEY (job_id, task_id),
  FOREIGN KEY(job_id) REFERENCES prepare_jobs(job_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_prepare_tasks_job ON prepare_tasks(job_id);
CREATE INDEX IF NOT EXISTS idx_prepare_tasks_status ON prepare_tasks(status);
CREATE INDEX IF NOT EXISTS idx_prepare_tasks_position ON prepare_tasks(job_id, position);

CREATE TABLE IF NOT EXISTS prepare_events (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL,
  type TEXT NOT NULL,
  ts TEXT NOT NULL,
  status TEXT,
  task_id TEXT,
  message TEXT,
  result_json TEXT,
  error_json TEXT,
  FOREIGN KEY(job_id) REFERENCES prepare_jobs(job_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_prepare_events_job_seq ON prepare_events(job_id, seq);
