CREATE TABLE bounty_applications (
    id BIGSERIAL PRIMARY KEY,
    bounty_id BIGINT NOT NULL,
    hunter_id BIGINT NOT NULL, -- 申请接单的用户ID
    status VARCHAR(50) NOT NULL DEFAULT 'APPLIED', -- 状态: APPLIED(已申请), ACCEPTED(已采纳), REJECTED(已拒绝)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 添加外键约束，悬赏删除时申请也级联删除
ALTER TABLE bounty_applications ADD CONSTRAINT fk_bounty_applications_bounty
    FOREIGN KEY (bounty_id) REFERENCES bounties (id) ON DELETE CASCADE;

-- 防止同一个用户对同一个悬赏重复申请
CREATE UNIQUE INDEX idx_unique_bounty_hunter ON bounty_applications (bounty_id, hunter_id);
CREATE INDEX idx_applications_hunter_id ON bounty_applications (hunter_id);