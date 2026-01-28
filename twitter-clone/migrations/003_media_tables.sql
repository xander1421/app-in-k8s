-- Migration: 003_media_tables.sql
-- Description: Create media storage and processing tables
-- Date: 2024

-- Create media table
CREATE TABLE IF NOT EXISTS media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('image', 'video', 'gif', 'audio')),
    url TEXT NOT NULL,
    thumbnail_url TEXT,
    
    -- Media properties
    width INT,
    height INT,
    duration INT, -- For video/audio in seconds
    size BIGINT NOT NULL, -- File size in bytes
    mime_type VARCHAR(100) NOT NULL,
    format VARCHAR(20), -- jpg, png, mp4, etc.
    
    -- Processing status
    processing_status VARCHAR(20) DEFAULT 'pending' 
        CHECK (processing_status IN ('pending', 'processing', 'completed', 'failed')),
    processing_error TEXT,
    processed_at TIMESTAMPTZ,
    
    -- Metadata
    alt_text TEXT, -- Accessibility
    blurhash VARCHAR(255), -- Placeholder for lazy loading
    color_palette JSONB, -- Dominant colors
    metadata JSONB, -- Additional metadata (EXIF, video info, etc.)
    
    -- Moderation
    is_sensitive BOOLEAN DEFAULT false,
    moderation_status VARCHAR(20) DEFAULT 'pending'
        CHECK (moderation_status IN ('pending', 'approved', 'rejected', 'flagged')),
    moderation_reason TEXT,
    moderated_at TIMESTAMPTZ,
    moderated_by UUID REFERENCES users(id),
    
    -- Storage
    storage_provider VARCHAR(50) DEFAULT 'minio', -- minio, s3, gcs, etc.
    storage_path TEXT NOT NULL,
    cdn_url TEXT,
    
    -- Stats
    view_count BIGINT DEFAULT 0,
    
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ -- Soft delete
);

-- Create media_variants table for different sizes/formats
CREATE TABLE IF NOT EXISTS media_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    variant_type VARCHAR(50) NOT NULL, -- thumbnail, small, medium, large, original
    url TEXT NOT NULL,
    width INT,
    height INT,
    size BIGINT,
    format VARCHAR(20),
    quality INT, -- 1-100 for compression quality
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create media_processing_jobs table
CREATE TABLE IF NOT EXISTS media_processing_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    job_type VARCHAR(50) NOT NULL, -- resize, transcode, thumbnail, moderate
    status VARCHAR(20) DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')),
    priority INT DEFAULT 5, -- 1-10, higher is more urgent
    attempts INT DEFAULT 0,
    max_attempts INT DEFAULT 3,
    
    -- Job details
    input_data JSONB,
    output_data JSONB,
    error_message TEXT,
    
    -- Timing
    scheduled_at TIMESTAMPTZ DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    
    -- Worker info
    worker_id VARCHAR(255),
    
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create tweet_media table (junction table)
CREATE TABLE IF NOT EXISTS tweet_media (
    tweet_id UUID NOT NULL,
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    position INT DEFAULT 0, -- Order of media in tweet
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (tweet_id, media_id)
);

-- Create dm_media table (junction table for direct messages)
CREATE TABLE IF NOT EXISTS dm_media (
    message_id UUID NOT NULL,
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    position INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (message_id, media_id)
);

-- Create media_analytics table
CREATE TABLE IF NOT EXISTS media_analytics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    
    -- Metrics
    impressions INT DEFAULT 0,
    engagements INT DEFAULT 0,
    clicks INT DEFAULT 0,
    shares INT DEFAULT 0,
    saves INT DEFAULT 0,
    
    -- Aggregated data
    total_watch_time INT, -- For videos, in seconds
    average_watch_duration INT,
    completion_rate DECIMAL(5,2), -- Percentage
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(media_id, date)
);

-- Create media_reports table
CREATE TABLE IF NOT EXISTS media_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id),
    reporter_id UUID NOT NULL REFERENCES users(id),
    reason VARCHAR(100) NOT NULL,
    details TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    reviewed_by UUID REFERENCES users(id),
    action_taken VARCHAR(50),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_media_user_id ON media(user_id);
CREATE INDEX IF NOT EXISTS idx_media_type ON media(type);
CREATE INDEX IF NOT EXISTS idx_media_processing_status ON media(processing_status);
CREATE INDEX IF NOT EXISTS idx_media_moderation_status ON media(moderation_status);
CREATE INDEX IF NOT EXISTS idx_media_created_at ON media(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_deleted_at ON media(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_sensitive ON media(is_sensitive);

CREATE INDEX IF NOT EXISTS idx_media_variants_media_id ON media_variants(media_id);
CREATE INDEX IF NOT EXISTS idx_media_variants_type ON media_variants(variant_type);

CREATE INDEX IF NOT EXISTS idx_media_processing_jobs_media_id ON media_processing_jobs(media_id);
CREATE INDEX IF NOT EXISTS idx_media_processing_jobs_status ON media_processing_jobs(status);
CREATE INDEX IF NOT EXISTS idx_media_processing_jobs_scheduled ON media_processing_jobs(scheduled_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_tweet_media_tweet_id ON tweet_media(tweet_id);
CREATE INDEX IF NOT EXISTS idx_tweet_media_media_id ON tweet_media(media_id);

CREATE INDEX IF NOT EXISTS idx_dm_media_message_id ON dm_media(message_id);
CREATE INDEX IF NOT EXISTS idx_dm_media_media_id ON dm_media(media_id);

CREATE INDEX IF NOT EXISTS idx_media_analytics_media_id ON media_analytics(media_id);
CREATE INDEX IF NOT EXISTS idx_media_analytics_date ON media_analytics(date);

CREATE INDEX IF NOT EXISTS idx_media_reports_media_id ON media_reports(media_id);
CREATE INDEX IF NOT EXISTS idx_media_reports_status ON media_reports(status);

-- Add trigger for updated_at
CREATE TRIGGER update_media_updated_at 
    BEFORE UPDATE ON media
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Function to increment media view count
CREATE OR REPLACE FUNCTION increment_media_view_count(p_media_id UUID)
RETURNS void AS $$
BEGIN
    UPDATE media 
    SET view_count = view_count + 1 
    WHERE id = p_media_id;
END;
$$ language 'plpgsql';

-- Function to clean up orphaned media
CREATE OR REPLACE FUNCTION cleanup_orphaned_media()
RETURNS void AS $$
BEGIN
    -- Mark media as deleted if not referenced anywhere
    UPDATE media 
    SET deleted_at = NOW()
    WHERE id NOT IN (
        SELECT DISTINCT media_id FROM tweet_media
        UNION
        SELECT DISTINCT media_id FROM dm_media
    )
    AND created_at < NOW() - INTERVAL '24 hours'
    AND deleted_at IS NULL;
END;
$$ language 'plpgsql';

-- Add comments
COMMENT ON TABLE media IS 'Media files uploaded by users';
COMMENT ON TABLE media_variants IS 'Different sizes and formats of media files';
COMMENT ON TABLE media_processing_jobs IS 'Background jobs for media processing';
COMMENT ON TABLE tweet_media IS 'Media attachments for tweets';
COMMENT ON TABLE dm_media IS 'Media attachments for direct messages';
COMMENT ON TABLE media_analytics IS 'Daily analytics for media performance';
COMMENT ON COLUMN media.blurhash IS 'Compact representation for image placeholders';
COMMENT ON COLUMN media.color_palette IS 'Dominant colors for UI theming';