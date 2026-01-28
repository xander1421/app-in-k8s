# Twitter Clone API Documentation

## Base URL
```
https://api.twitter-clone.com
```

## Authentication
All authenticated endpoints require a Bearer token in the Authorization header:
```
Authorization: Bearer <access_token>
```

## Services Overview

### 1. User Service (Port 8080)
Handles user management, authentication, and social features.

### 2. Tweet Service (Port 8081)
Manages tweets, likes, retweets, and bookmarks.

### 3. Timeline Service (Port 8082)
Generates and manages user timelines with smart strategies.

### 4. Media Service (Port 8083)
Handles media upload, processing, and delivery.

### 5. Notification Service (Port 8084)
Manages user notifications and preferences.

### 6. Search Service (Port 8085)
Provides search functionality for tweets and users.

### 7. Fanout Service (Worker)
Background worker for message fanout and processing.

### 8. Realtime Service (Port 8086)
SSE (Server-Sent Events) server for real-time updates over HTTP/3.

---

## API Endpoints

### Authentication

#### Register
```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "username": "johndoe",
  "email": "john@example.com",
  "password": "SecureP@ss123",
  "name": "John Doe"
}

Response: 201 Created
{
  "user": {
    "id": "uuid",
    "username": "johndoe",
    "email": "john@example.com",
    "name": "John Doe",
    "created_at": "2024-01-01T00:00:00Z"
  },
  "access_token": "jwt_token",
  "refresh_token": "refresh_token",
  "expires_in": 900
}
```

#### Login
```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "john@example.com",
  "password": "SecureP@ss123"
}

Response: 200 OK
{
  "user": {...},
  "access_token": "jwt_token",
  "refresh_token": "refresh_token",
  "expires_in": 900
}
```

#### Refresh Token
```http
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "refresh_token"
}

Response: 200 OK
{
  "access_token": "new_jwt_token",
  "expires_in": 900
}
```

#### Logout
```http
POST /api/v1/auth/logout
Authorization: Bearer <token>

Response: 204 No Content
```

### Users

#### Get User Profile
```http
GET /api/v1/users/{userID}
Authorization: Bearer <token>

Response: 200 OK
{
  "id": "uuid",
  "username": "johndoe",
  "name": "John Doe",
  "bio": "Software developer",
  "avatar_url": "https://...",
  "follower_count": 150,
  "following_count": 75,
  "tweet_count": 320,
  "created_at": "2024-01-01T00:00:00Z"
}
```

#### Update Profile
```http
PUT /api/v1/users/profile
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "John Doe",
  "bio": "Updated bio",
  "location": "San Francisco",
  "website": "https://johndoe.com"
}

Response: 200 OK
{
  "user": {...}
}
```

#### Follow User
```http
POST /api/v1/users/{userID}/follow
Authorization: Bearer <token>

Response: 204 No Content
```

#### Unfollow User
```http
DELETE /api/v1/users/{userID}/follow
Authorization: Bearer <token>

Response: 204 No Content
```

#### Get Followers
```http
GET /api/v1/users/{userID}/followers?limit=20&cursor=
Authorization: Bearer <token>

Response: 200 OK
{
  "users": [...],
  "next_cursor": "cursor_string",
  "has_more": true
}
```

#### Get Following
```http
GET /api/v1/users/{userID}/following?limit=20&cursor=
Authorization: Bearer <token>

Response: 200 OK
{
  "users": [...],
  "next_cursor": "cursor_string",
  "has_more": true
}
```

### Tweets

#### Create Tweet
```http
POST /api/v1/tweets
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": "Hello Twitter Clone! 🚀",
  "media_ids": ["media_id1", "media_id2"],
  "reply_to_id": null
}

Response: 201 Created
{
  "id": "tweet_uuid",
  "author_id": "user_uuid",
  "content": "Hello Twitter Clone! 🚀",
  "media_urls": [...],
  "like_count": 0,
  "retweet_count": 0,
  "reply_count": 0,
  "created_at": "2024-01-01T00:00:00Z"
}
```

#### Get Tweet
```http
GET /api/v1/tweets/{tweetID}
Authorization: Bearer <token>

Response: 200 OK
{
  "id": "tweet_uuid",
  "author": {
    "id": "user_uuid",
    "username": "johndoe",
    "name": "John Doe",
    "avatar_url": "..."
  },
  "content": "Tweet content",
  "media_urls": [...],
  "like_count": 42,
  "retweet_count": 10,
  "reply_count": 5,
  "is_liked": false,
  "is_retweeted": false,
  "is_bookmarked": false,
  "created_at": "2024-01-01T00:00:00Z"
}
```

#### Delete Tweet
```http
DELETE /api/v1/tweets/{tweetID}
Authorization: Bearer <token>

Response: 204 No Content
```

#### Like Tweet
```http
POST /api/v1/tweets/{tweetID}/like
Authorization: Bearer <token>

Response: 204 No Content
```

#### Unlike Tweet
```http
DELETE /api/v1/tweets/{tweetID}/like
Authorization: Bearer <token>

Response: 204 No Content
```

#### Retweet
```http
POST /api/v1/tweets/{tweetID}/retweet
Authorization: Bearer <token>

Response: 201 Created
{
  "id": "retweet_uuid",
  "retweet_of": {...},
  "created_at": "2024-01-01T00:00:00Z"
}
```

#### Get Replies
```http
GET /api/v1/tweets/{tweetID}/replies?limit=20&cursor=
Authorization: Bearer <token>

Response: 200 OK
{
  "tweets": [...],
  "next_cursor": "cursor_string",
  "has_more": true
}
```

### Bookmarks

#### Add Bookmark
```http
POST /api/v1/tweets/{tweetID}/bookmark
Authorization: Bearer <token>

Response: 204 No Content
```

#### Remove Bookmark
```http
DELETE /api/v1/tweets/{tweetID}/bookmark
Authorization: Bearer <token>

Response: 204 No Content
```

#### Get Bookmarks
```http
GET /api/v1/bookmarks?limit=20&cursor=
Authorization: Bearer <token>

Response: 200 OK
{
  "tweets": [...],
  "next_cursor": "cursor_string",
  "has_more": true
}
```

### Timeline

#### Home Timeline
```http
GET /api/v1/timeline/home?limit=20&cursor=
Authorization: Bearer <token>

Response: 200 OK
{
  "tweets": [...],
  "next_cursor": "cursor_string",
  "has_more": true,
  "strategy_used": "push|pull|hybrid"
}
```

#### User Timeline
```http
GET /api/v1/timeline/user/{userID}?limit=20&cursor=
Authorization: Bearer <token>

Response: 200 OK
{
  "tweets": [...],
  "next_cursor": "cursor_string",
  "has_more": true
}
```

#### Trending Timeline
```http
GET /api/v1/timeline/trending?limit=20
Authorization: Bearer <token>

Response: 200 OK
{
  "tweets": [...],
  "has_more": false
}
```

### Media

#### Upload Media
```http
POST /api/v1/media/upload
Authorization: Bearer <token>
Content-Type: multipart/form-data

file: <binary>
alt_text: "Description of image"

Response: 201 Created
{
  "id": "media_uuid",
  "type": "image",
  "url": "https://cdn.../image.jpg",
  "thumbnail_url": "https://cdn.../thumb.jpg",
  "processing_status": "processing",
  "width": 1920,
  "height": 1080,
  "size": 2048000
}
```

#### Get Media Status
```http
GET /api/v1/media/{mediaID}
Authorization: Bearer <token>

Response: 200 OK
{
  "id": "media_uuid",
  "type": "image",
  "url": "https://cdn.../image.jpg",
  "processing_status": "completed",
  "variants": [
    {
      "type": "thumbnail",
      "url": "...",
      "width": 150,
      "height": 150
    },
    {
      "type": "small",
      "url": "...",
      "width": 480,
      "height": 480
    }
  ]
}
```

### Notifications

#### Get Notifications
```http
GET /api/v1/notifications?limit=20&cursor=
Authorization: Bearer <token>

Response: 200 OK
{
  "notifications": [
    {
      "id": "notif_uuid",
      "type": "like|follow|mention|retweet|reply",
      "actor": {
        "id": "user_uuid",
        "username": "johndoe",
        "name": "John Doe",
        "avatar_url": "..."
      },
      "tweet": {...},
      "message": "John Doe liked your tweet",
      "read": false,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "next_cursor": "cursor_string",
  "has_more": true,
  "unread_count": 5
}
```

#### Mark as Read
```http
PUT /api/v1/notifications/{notificationID}/read
Authorization: Bearer <token>

Response: 204 No Content
```

#### Mark All as Read
```http
PUT /api/v1/notifications/read-all
Authorization: Bearer <token>

Response: 204 No Content
```

### Search

#### Search Tweets
```http
GET /api/v1/search/tweets?q=query&limit=20
Authorization: Bearer <token>

Response: 200 OK
{
  "tweets": [...],
  "total": 150,
  "has_more": true
}
```

#### Search Users
```http
GET /api/v1/search/users?q=query&limit=20
Authorization: Bearer <token>

Response: 200 OK
{
  "users": [...],
  "total": 42,
  "has_more": true
}
```

#### Search Hashtags
```http
GET /api/v1/search/hashtags?q=query
Authorization: Bearer <token>

Response: 200 OK
{
  "hashtags": [
    {
      "tag": "#golang",
      "tweet_count": 1250
    }
  ]
}
```

### WebSocket (Realtime)

#### Connect
```
wss://api.twitter-clone.com/ws?token=jwt_token
```

#### Message Types

**Subscribe to Channel**
```json
{
  "type": "subscribe",
  "channel": "user:notifications",
  "data": {}
}
```

**Receive Notification**
```json
{
  "type": "notification",
  "data": {
    "id": "notif_uuid",
    "type": "like",
    "message": "John liked your tweet"
  },
  "timestamp": 1704067200
}
```

**Timeline Update**
```json
{
  "type": "timeline_update",
  "data": {
    "action": "new_tweet",
    "tweet": {...}
  },
  "timestamp": 1704067200
}
```

**Presence Update**
```json
{
  "type": "presence",
  "data": {
    "user_id": "uuid",
    "status": "online|offline|away"
  },
  "timestamp": 1704067200
}
```

---

## Error Responses

### Standard Error Format
```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": {}
  }
}
```

### Common HTTP Status Codes

- `200 OK` - Request succeeded
- `201 Created` - Resource created
- `204 No Content` - Request succeeded, no content to return
- `400 Bad Request` - Invalid request data
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Access denied
- `404 Not Found` - Resource not found
- `409 Conflict` - Conflict with current state
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

---

## Rate Limits

| Service | Limit | Window |
|---------|-------|--------|
| User Service | 100 req/min | 1 minute |
| Tweet Service | 300 req/min | 1 minute |
| Timeline Service | 200 req/min | 1 minute |
| Media Service | 50 req/min | 1 minute |
| Search Service | 60 req/min | 1 minute |

Rate limit headers:
```
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 299
X-RateLimit-Reset: 1704067200
```

---

## Pagination

Use cursor-based pagination for all list endpoints:

```
GET /api/v1/endpoint?limit=20&cursor=eyJpZCI6MTIzfQ==
```

Response includes:
```json
{
  "data": [...],
  "next_cursor": "eyJpZCI6MTQzfQ==",
  "has_more": true
}
```

---

## Webhooks (Optional)

Configure webhooks to receive real-time events:

```http
POST /api/v1/webhooks
Authorization: Bearer <token>
Content-Type: application/json

{
  "url": "https://your-server.com/webhook",
  "events": ["tweet.created", "follow.created"],
  "secret": "webhook_secret"
}
```

Events are sent as POST requests with signature verification:
```
X-Webhook-Signature: sha256=...
```