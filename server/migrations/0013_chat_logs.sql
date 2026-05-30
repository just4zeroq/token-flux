-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS llm_chat_logs (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL DEFAULT 0,
    api_key_id       BIGINT NOT NULL DEFAULT 0,
    model_spec_id    BIGINT NOT NULL DEFAULT 0,
    request_id       VARCHAR(64) NOT NULL DEFAULT '',
    capability       VARCHAR(32) NOT NULL DEFAULT '',
    is_stream        BOOLEAN NOT NULL DEFAULT false,
    request_body     JSONB NOT NULL DEFAULT '{}',
    response_body    JSONB NOT NULL DEFAULT '{}',
    prompt_tokens    INT NOT NULL DEFAULT 0,
    completion_tokens INT NOT NULL DEFAULT 0,
    total_tokens     INT NOT NULL DEFAULT 0,
    cost_credits     BIGINT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_logs_user_id ON llm_chat_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_logs_request_id ON llm_chat_logs(request_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS llm_chat_logs;

-- +goose StatementEnd
