-- Migration: 002_social_features.sql
-- Description: Add social features (blocks, mutes, bookmarks, lists)
-- Date: 2024

-- Create blocks table
CREATE TABLE IF NOT EXISTS blocks (
    blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id != blocked_id)
);

-- Create mutes table
CREATE TABLE IF NOT EXISTS mutes (
    muter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    muted_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mute_until TIMESTAMPTZ, -- NULL for permanent mute
    mute_retweets BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (muter_id, muted_id),
    CHECK (muter_id != muted_id)
);

-- Create bookmarks table
CREATE TABLE IF NOT EXISTS bookmarks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tweet_id UUID NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, tweet_id)
);

-- Create lists table
CREATE TABLE IF NOT EXISTS lists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_private BOOLEAN DEFAULT false,
    member_count INT DEFAULT 0,
    subscriber_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create list_members table
CREATE TABLE IF NOT EXISTS list_members (
    list_id UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    added_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (list_id, user_id)
);

-- Create list_subscribers table
CREATE TABLE IF NOT EXISTS list_subscribers (
    list_id UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (list_id, user_id)
);

-- Create user_settings table
CREATE TABLE IF NOT EXISTS user_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- Notification settings
    notifications_enabled BOOLEAN DEFAULT true,
    email_notifications BOOLEAN DEFAULT true,
    push_notifications BOOLEAN DEFAULT false,
    sms_notifications BOOLEAN DEFAULT false,
    -- Privacy settings
    private_account BOOLEAN DEFAULT false,
    show_activity_status BOOLEAN DEFAULT true,
    allow_dm_from_anyone BOOLEAN DEFAULT false,
    allow_dm_requests BOOLEAN DEFAULT true,
    show_read_receipts BOOLEAN DEFAULT true,
    protect_tweets BOOLEAN DEFAULT false,
    -- Content settings
    sensitive_content_filter BOOLEAN DEFAULT true,
    quality_filter BOOLEAN DEFAULT true,
    muted_words TEXT[], -- Array of muted words/phrases
    -- Display settings
    language VARCHAR(10) DEFAULT 'en',
    timezone VARCHAR(50) DEFAULT 'UTC',
    theme VARCHAR(20) DEFAULT 'light',
    -- Limits
    daily_tweet_limit INT DEFAULT 2400,
    daily_follow_limit INT DEFAULT 400,
    daily_dm_limit INT DEFAULT 1000,
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create report_reasons table
CREATE TABLE IF NOT EXISTS report_reasons (
    id SERIAL PRIMARY KEY,
    category VARCHAR(50) NOT NULL,
    reason VARCHAR(100) NOT NULL,
    description TEXT,
    requires_details BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true
);

-- Create user_reports table
CREATE TABLE IF NOT EXISTS user_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID NOT NULL REFERENCES users(id),
    reported_user_id UUID REFERENCES users(id),
    reported_tweet_id UUID,
    reason_id INT REFERENCES report_reasons(id),
    details TEXT,
    status VARCHAR(20) DEFAULT 'pending', -- pending, reviewing, resolved, dismissed
    moderator_id UUID REFERENCES users(id),
    moderator_notes TEXT,
    action_taken VARCHAR(50), -- warning, suspend, ban, none
    created_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    CHECK (reported_user_id IS NOT NULL OR reported_tweet_id IS NOT NULL)
);

-- Create user_suspensions table
CREATE TABLE IF NOT EXISTS user_suspensions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    reason VARCHAR(255) NOT NULL,
    details TEXT,
    suspended_by UUID REFERENCES users(id),
    suspended_until TIMESTAMPTZ,
    is_permanent BOOLEAN DEFAULT false,
    appeal_status VARCHAR(20), -- none, pending, approved, denied
    appeal_reason TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    lifted_at TIMESTAMPTZ
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_blocks_blocker ON blocks(blocker_id);
CREATE INDEX IF NOT EXISTS idx_blocks_blocked ON blocks(blocked_id);
CREATE INDEX IF NOT EXISTS idx_mutes_muter ON mutes(muter_id);
CREATE INDEX IF NOT EXISTS idx_mutes_muted ON mutes(muted_id);
CREATE INDEX IF NOT EXISTS idx_mutes_until ON mutes(mute_until) WHERE mute_until IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_bookmarks_user_id ON bookmarks(user_id);
CREATE INDEX IF NOT EXISTS idx_bookmarks_tweet_id ON bookmarks(tweet_id);
CREATE INDEX IF NOT EXISTS idx_bookmarks_created_at ON bookmarks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_lists_owner_id ON lists(owner_id);
CREATE INDEX IF NOT EXISTS idx_lists_private ON lists(is_private);
CREATE INDEX IF NOT EXISTS idx_list_members_list_id ON list_members(list_id);
CREATE INDEX IF NOT EXISTS idx_list_members_user_id ON list_members(user_id);
CREATE INDEX IF NOT EXISTS idx_list_subscribers_list_id ON list_subscribers(list_id);
CREATE INDEX IF NOT EXISTS idx_list_subscribers_user_id ON list_subscribers(user_id);
CREATE INDEX IF NOT EXISTS idx_user_reports_reporter ON user_reports(reporter_id);
CREATE INDEX IF NOT EXISTS idx_user_reports_reported_user ON user_reports(reported_user_id);
CREATE INDEX IF NOT EXISTS idx_user_reports_status ON user_reports(status);
CREATE INDEX IF NOT EXISTS idx_user_suspensions_user_id ON user_suspensions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_suspensions_active ON user_suspensions(user_id) 
    WHERE lifted_at IS NULL;

-- Add triggers for updated_at
CREATE TRIGGER update_lists_updated_at 
    BEFORE UPDATE ON lists
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_settings_updated_at 
    BEFORE UPDATE ON user_settings
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Insert default report reasons
INSERT INTO report_reasons (category, reason, description, requires_details) VALUES
    ('spam', 'Spam', 'Posting spam or malicious links', false),
    ('abuse', 'Abusive or harmful', 'Engaging in targeted harassment', true),
    ('hate', 'Hate speech', 'Promoting hate based on race, ethnicity, etc.', true),
    ('violence', 'Violence', 'Threatening or promoting violence', true),
    ('privacy', 'Private information', 'Posting private information', true),
    ('impersonation', 'Impersonation', 'Pretending to be someone else', true),
    ('copyright', 'Copyright violation', 'Violating intellectual property rights', true),
    ('inappropriate', 'Inappropriate content', 'Posting inappropriate or sensitive media', false),
    ('self_harm', 'Self-harm or suicide', 'Promoting self-harm or suicide', false),
    ('misinformation', 'Misinformation', 'Spreading false or misleading information', true)
ON CONFLICT DO NOTHING;

-- Function to update list member count
CREATE OR REPLACE FUNCTION update_list_member_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE lists SET member_count = member_count + 1 WHERE id = NEW.list_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE lists SET member_count = member_count - 1 WHERE id = OLD.list_id;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

-- Add trigger for list member count
CREATE TRIGGER update_list_member_count_trigger
    AFTER INSERT OR DELETE ON list_members
    FOR EACH ROW
    EXECUTE FUNCTION update_list_member_count();

-- Add comments
COMMENT ON TABLE blocks IS 'User blocking relationships';
COMMENT ON TABLE mutes IS 'User muting relationships with optional expiration';
COMMENT ON TABLE bookmarks IS 'Saved tweets for later reading';
COMMENT ON TABLE lists IS 'User-created lists for organizing accounts';
COMMENT ON TABLE user_settings IS 'User preferences and settings';
COMMENT ON TABLE user_reports IS 'Content moderation reports';
COMMENT ON TABLE user_suspensions IS 'Account suspension records';