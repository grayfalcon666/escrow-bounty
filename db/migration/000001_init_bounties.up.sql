CREATE TABLE bounties (
    id BIGSERIAL PRIMARY KEY,
    employer_id BIGINT NOT NULL, -- 关联用户的ID
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    reward_amount BIGINT NOT NULL, -- 悬赏金额，建议用最小货币单位(如分)避免浮点数精度问题
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- 状态: PENDING, IN_PROGRESS, COMPLETED, CANCELED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bounties_employer_id ON bounties (employer_id); -- 发布者查自己的悬赏
CREATE INDEX idx_bounties_status ON bounties (status); --按状态筛选悬赏