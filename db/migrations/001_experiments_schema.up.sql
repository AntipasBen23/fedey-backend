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

CREATE TABLE IF NOT EXISTS content_drafts (
    id TEXT PRIMARY KEY,
    channel TEXT NOT NULL,
    hook TEXT NOT NULL,
    body TEXT NOT NULL,
    rationale TEXT NOT NULL,
    source_trend TEXT NOT NULL,
    experiment_id TEXT NULL REFERENCES experiments(id),
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_drafts_created_at ON content_drafts(created_at DESC);

CREATE TABLE IF NOT EXISTS publishing_schedules (
    id TEXT PRIMARY KEY,
    draft_id TEXT NOT NULL REFERENCES content_drafts(id) ON DELETE CASCADE,
    variant_label TEXT NULL,
    channel TEXT NOT NULL,
    queue_profile TEXT NOT NULL DEFAULT 'standard',
    platform_post_id TEXT NULL,
    scheduled_for TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL,
    published_at TIMESTAMPTZ NULL,
    performance_synced_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_publishing_schedules_scheduled_for ON publishing_schedules(scheduled_for ASC);
ALTER TABLE publishing_schedules ADD COLUMN IF NOT EXISTS performance_synced_at TIMESTAMPTZ NULL;
ALTER TABLE publishing_schedules ADD COLUMN IF NOT EXISTS queue_profile TEXT NOT NULL DEFAULT 'standard';

CREATE TABLE IF NOT EXISTS community_inbox (
    id TEXT PRIMARY KEY,
    platform TEXT NOT NULL,
    author TEXT NOT NULL,
    message TEXT NOT NULL,
    sentiment TEXT NOT NULL,
    reply_draft TEXT NULL,
    linked_post_ref TEXT NOT NULL,
    external_comment_id TEXT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    replied_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_community_inbox_created_at ON community_inbox(created_at DESC);

CREATE TABLE IF NOT EXISTS onboarding_sessions (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    job_description TEXT NOT NULL,
    account_mode TEXT NOT NULL,
    objective TEXT NOT NULL,
    primary_platform TEXT NOT NULL,
    brand_name TEXT NOT NULL,
    audience TEXT NOT NULL,
    voice_summary TEXT NOT NULL,
    constraints TEXT[] NOT NULL DEFAULT '{}',
    review_mode TEXT NOT NULL DEFAULT 'auto',
    approval_status TEXT NOT NULL DEFAULT 'not_required',
    audit JSONB NOT NULL DEFAULT '{}'::jsonb,
    activation JSONB NOT NULL DEFAULT '{}'::jsonb,
    history JSONB NOT NULL DEFAULT '[]'::jsonb,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE onboarding_sessions ADD COLUMN IF NOT EXISTS activation JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE onboarding_sessions ADD COLUMN IF NOT EXISTS review_mode TEXT NOT NULL DEFAULT 'auto';
ALTER TABLE onboarding_sessions ADD COLUMN IF NOT EXISTS approval_status TEXT NOT NULL DEFAULT 'not_required';
ALTER TABLE onboarding_sessions ADD COLUMN IF NOT EXISTS history JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS onboarding_questions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES onboarding_sessions(id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    category TEXT NOT NULL,
    answer TEXT NULL,
    required BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    answered_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_onboarding_questions_session_id ON onboarding_questions(session_id);

CREATE TABLE IF NOT EXISTS automation_runs (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    drafts_generated INTEGER NOT NULL,
    schedules_created INTEGER NOT NULL,
    posts_published INTEGER NOT NULL DEFAULT 0,
    signals_ingested INTEGER NOT NULL DEFAULT 0,
    mentions_synced INTEGER NOT NULL,
    replies_drafted INTEGER NOT NULL,
    triggered_by TEXT NOT NULL,
    notes TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_automation_runs_created_at ON automation_runs(created_at DESC);

CREATE TABLE IF NOT EXISTS x_accounts (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL,
    username TEXT NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    token_type TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS x_oauth_states (
    state TEXT PRIMARY KEY,
    code_verifier TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS linkedin_accounts (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL UNIQUE,
    member_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    author_urn TEXT NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    expires_at TIMESTAMPTZ NOT NULL,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS linkedin_oauth_states (
    state TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform_performance_snapshots (
    id TEXT PRIMARY KEY,
    platform TEXT NOT NULL,
    external_post_id TEXT NOT NULL,
    author_ref TEXT NOT NULL,
    content_preview TEXT NOT NULL,
    like_count INTEGER NOT NULL DEFAULT 0,
    reply_count INTEGER NOT NULL DEFAULT 0,
    quote_count INTEGER NOT NULL DEFAULT 0,
    comment_count INTEGER NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_platform_performance_platform_captured_at
    ON platform_performance_snapshots(platform, captured_at DESC);

ALTER TABLE platform_performance_snapshots ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

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
