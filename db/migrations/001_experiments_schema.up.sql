CREATE TABLE IF NOT EXISTS hypotheses (
    id TEXT PRIMARY KEY,
    statement TEXT NOT NULL,
    channel TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS experiments (
    id TEXT PRIMARY KEY,
    hypothesis_id TEXT NOT NULL,
    metric TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('draft', 'running', 'completed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_experiments_hypothesis_id ON experiments(hypothesis_id);
CREATE INDEX IF NOT EXISTS idx_experiments_created_at ON experiments(created_at DESC);

CREATE TABLE IF NOT EXISTS experiment_variants (
    id TEXT PRIMARY KEY,
    experiment_id TEXT NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_experiment_variants_experiment_id ON experiment_variants(experiment_id);
