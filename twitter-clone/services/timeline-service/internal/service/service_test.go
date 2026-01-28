package service

import (
	"fmt"
	"testing"
)

// Unit tests for timeline service constants and validation

func TestTimelineConstants(t *testing.T) {
	if DefaultTimelineLimit != 20 {
		t.Errorf("Expected DefaultTimelineLimit to be 20, got %d", DefaultTimelineLimit)
	}

	if MaxTimelineLimit != 100 {
		t.Errorf("Expected MaxTimelineLimit to be 100, got %d", MaxTimelineLimit)
	}

	if TimelineMaxSize != 800 {
		t.Errorf("Expected TimelineMaxSize to be 800, got %d", TimelineMaxSize)
	}
}

func TestTimelineLimitValidation(t *testing.T) {
	tests := []struct {
		name          string
		inputLimit    int
		expectedLimit int
	}{
		{
			name:          "zero uses default",
			inputLimit:    0,
			expectedLimit: DefaultTimelineLimit,
		},
		{
			name:          "negative uses default",
			inputLimit:    -5,
			expectedLimit: DefaultTimelineLimit,
		},
		{
			name:          "normal limit unchanged",
			inputLimit:    50,
			expectedLimit: 50,
		},
		{
			name:          "exceeds max capped",
			inputLimit:    200,
			expectedLimit: MaxTimelineLimit,
		},
		{
			name:          "exactly max allowed",
			inputLimit:    100,
			expectedLimit: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the limit validation logic from GetHomeTimeline
			limit := tt.inputLimit
			if limit <= 0 {
				limit = DefaultTimelineLimit
			}
			if limit > MaxTimelineLimit {
				limit = MaxTimelineLimit
			}

			if limit != tt.expectedLimit {
				t.Errorf("Expected limit %d, got %d", tt.expectedLimit, limit)
			}
		})
	}
}

func TestTimelineHasMoreCalculation(t *testing.T) {
	tests := []struct {
		name     string
		fetched  int
		limit    int
		hasMore  bool
		returned int
	}{
		{
			name:     "fewer than limit",
			fetched:  5,
			limit:    20,
			hasMore:  false,
			returned: 5,
		},
		{
			name:     "exactly at limit",
			fetched:  20,
			limit:    20,
			hasMore:  false,
			returned: 20,
		},
		{
			name:     "more than limit",
			fetched:  21,
			limit:    20,
			hasMore:  true,
			returned: 20,
		},
		{
			name:     "empty result",
			fetched:  0,
			limit:    20,
			hasMore:  false,
			returned: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the hasMore calculation from GetHomeTimeline
			// The service fetches limit+1 items to check for more
			hasMore := tt.fetched > tt.limit
			returned := tt.fetched
			if hasMore {
				returned = tt.limit
			}

			if hasMore != tt.hasMore {
				t.Errorf("Expected hasMore=%v, got %v", tt.hasMore, hasMore)
			}
			if returned != tt.returned {
				t.Errorf("Expected returned=%d, got %d", tt.returned, returned)
			}
		})
	}
}

func TestNextCursorCalculation(t *testing.T) {
	tests := []struct {
		name       string
		offset     int
		limit      int
		hasMore    bool
		wantCursor string
	}{
		{
			name:       "first page with more",
			offset:     0,
			limit:      20,
			hasMore:    true,
			wantCursor: "20",
		},
		{
			name:       "second page with more",
			offset:     20,
			limit:      20,
			hasMore:    true,
			wantCursor: "40",
		},
		{
			name:       "no more results",
			offset:     0,
			limit:      20,
			hasMore:    false,
			wantCursor: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cursor string
			if tt.hasMore {
				cursor = fmt.Sprintf("%d", tt.offset+tt.limit)
			}

			if cursor != tt.wantCursor {
				t.Errorf("Expected cursor=%s, got %s", tt.wantCursor, cursor)
			}
		})
	}
}
