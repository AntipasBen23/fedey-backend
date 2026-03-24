CREATE TABLE IF NOT EXISTS hypotheses (
    id TEXT PRIMARY KEY,
    statement TEXT NOT NULL,
    channel TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS brand_memory (
    id TEXT PRIMARY KEY,
    brand_name TEXT NOT NULL,
    tone TEXT NOT NULL,
    audience TEXT NOT NULL,
    pillars TEXT[] NOT NULL DEFAULT '{}',
    guardrails TEXT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS trend_signals (
    id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    source TEXT NOT NULL,
    angle TEXT NOT NULL,
    velocity INTEGER NOT NULL CHECK (velocity >= 0),
    relevance DOUBLE PRECISION NOT NULL CHECK (relevance >= 0 AND relevance <= 1),
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_trend_signals_relevance ON trend_signals(relevance DESC, observed_at DESC);

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

CREATE TABLE IF NOT EXISTS analytics_events (
    id TEXT PRIMARY KEY,
    experiment_id TEXT NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    variant TEXT NOT NULL,
    value DOUBLE PRECISION NOT NULL CHECK (value >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_analytics_events_experiment_id ON analytics_events(experiment_id);
CREATE INDEX IF NOT EXISTS idx_analytics_events_variant ON analytics_events(variant);
