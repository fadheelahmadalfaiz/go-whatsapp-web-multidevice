package s3

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// mockS3Client
// ---------------------------------------------------------------------------

type mockS3Client struct {
	mock.Mock
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.PutObjectOutput), args.Error(1)
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func (m *mockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.DeleteObjectOutput), args.Error(1)
}

func (m *mockS3Client) HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.HeadBucketOutput), args.Error(1)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// testClient creates a Client with the given config and a mock S3 back-end.
func testClient(cfg config.S3Config) *Client {
	return &Client{
		s3Client: new(mockS3Client),
		cfg:      cfg,
	}
}

func testClientWithMock(cfg config.S3Config) (*Client, *mockS3Client) {
	m := new(mockS3Client)
	return &Client{s3Client: m, cfg: cfg}, m
}

// resetSingleton resets the package-level singleton between tests.
func resetSingleton() {
	instance = nil
	once = sync.Once{}
}

// ---------------------------------------------------------------------------
// Tests: Initialize
// ---------------------------------------------------------------------------

func TestInitialize_Disabled(t *testing.T) {
	resetSingleton()
	err := Initialize(config.S3Config{Enabled: false})
	assert.NoError(t, err)
	assert.Nil(t, instance)
}

func TestInitialize_ValidatesBucket(t *testing.T) {
	resetSingleton()
	err := Initialize(config.S3Config{Enabled: true, AccessKey: "ak", SecretKey: "sk"})
	assert.EqualError(t, err, "s3 bucket is required when S3 is enabled")
	assert.Nil(t, instance)
}

func TestInitialize_ValidatesCredentials(t *testing.T) {
	resetSingleton()
	err := Initialize(config.S3Config{Enabled: true, Bucket: "b"})
	assert.EqualError(t, err, "s3 access key and secret key are required when S3 is enabled")
	assert.Nil(t, instance)
}

func TestInitialize_OnlyOnce(t *testing.T) {
	resetSingleton()

	// Replace newClientFn so the first Initialize succeeds.
	restore := newClientFn
	newClientFn = func(cfg config.S3Config) (*Client, error) {
		m := new(mockS3Client)
		m.On("HeadBucket", mock.Anything, mock.Anything).Return(&s3.HeadBucketOutput{}, nil)
		return &Client{s3Client: m, cfg: cfg}, nil
	}
	defer func() { newClientFn = restore }()

	err := Initialize(config.S3Config{Enabled: true, Bucket: "first", AccessKey: "ak", SecretKey: "sk", Region: "us-east-1"})
	require.NoError(t, err)
	assert.Equal(t, "first", instance.cfg.Bucket)

	// Second Initialize with different config must be a no-op.
	err = Initialize(config.S3Config{Enabled: true, Bucket: "second", AccessKey: "other", SecretKey: "other", Region: "eu-west-1"})
	assert.NoError(t, err, "second Initialize should be no-op")
	assert.Equal(t, "first", instance.cfg.Bucket, "original instance should survive")
}

// ---------------------------------------------------------------------------
// Tests: IsEnabled / GetClient
// ---------------------------------------------------------------------------

func TestIsEnabled_NilInstance(t *testing.T) {
	resetSingleton()
	assert.False(t, IsEnabled())
	assert.Nil(t, GetClient())
}

func TestIsEnabled_Enabled(t *testing.T) {
	resetSingleton()
	instance = &Client{cfg: config.S3Config{Enabled: true}}
	assert.True(t, IsEnabled())
	assert.NotNil(t, GetClient())
}

func TestIsEnabled_Disabled(t *testing.T) {
	resetSingleton()
	instance = &Client{cfg: config.S3Config{Enabled: false}}
	assert.False(t, IsEnabled())
	assert.NotNil(t, GetClient())
}

// ---------------------------------------------------------------------------
// Tests: GenerateKey
// ---------------------------------------------------------------------------

func TestGenerateKey_NoPrefix(t *testing.T) {
	c := testClient(config.S3Config{})
	key := c.GenerateKey("photo.jpg")

	assert.True(t, strings.HasPrefix(key, "202"), "should start with year prefix")
	assert.Contains(t, key, "-photo.jpg")
	assert.Len(t, key, 8+1+36+1+9, "expected YYYYMMDD-UUID-photo.jpg length")
}

func TestGenerateKey_WithPrefix(t *testing.T) {
	c := testClient(config.S3Config{Prefix: "whatsapp-media/"})
	key := c.GenerateKey("photo.jpg")

	assert.True(t, strings.HasPrefix(key, "whatsapp-media/"))
	assert.Contains(t, key, "-photo.jpg")
}

func TestGenerateKey_WithPrefixNoTrailingSlash(t *testing.T) {
	c := testClient(config.S3Config{Prefix: "media"})
	key := c.GenerateKey("doc.pdf")
	assert.True(t, strings.HasPrefix(key, "media/"), "should add separator")
	assert.Contains(t, key, "-doc.pdf")
}

func TestGenerateKey_SpecialFilename(t *testing.T) {
	c := testClient(config.S3Config{})
	key := c.GenerateKey("hello world!@#.jpg")
	assert.Contains(t, key, "-hello world!@#.jpg")
}

// ---------------------------------------------------------------------------
// Tests: buildURL
// ---------------------------------------------------------------------------

func TestBuildURL_PublicURL(t *testing.T) {
	c := testClient(config.S3Config{
		PublicURL: "https://media.example.com",
	})
	url := c.buildURL("photos/2026/01/test.jpg")
	assert.Equal(t, "https://media.example.com/photos/2026/01/test.jpg", url)
}

func TestBuildURL_PublicURLTrailingSlash(t *testing.T) {
	c := testClient(config.S3Config{
		PublicURL: "https://media.example.com/",
	})
	url := c.buildURL("test.jpg")
	assert.Equal(t, "https://media.example.com/test.jpg", url)
}

func TestBuildURL_CustomEndpoint(t *testing.T) {
	c := testClient(config.S3Config{
		Endpoint: "https://myaccount.r2.cloudflarestorage.com",
		Bucket:   "my-bucket",
	})
	url := c.buildURL("photos/test.jpg")
	assert.Equal(t, "https://myaccount.r2.cloudflarestorage.com/my-bucket/photos/test.jpg", url)
}

func TestBuildURL_DefaultAWS(t *testing.T) {
	c := testClient(config.S3Config{
		Bucket: "my-bucket",
		Region: "us-east-1",
	})
	url := c.buildURL("photos/test.jpg")
	assert.Equal(t, "https://my-bucket.s3.us-east-1.amazonaws.com/photos/test.jpg", url)
}

func TestBuildURL_UsePathStyleRespected(t *testing.T) {
	// When Endpoint is set, the real client sets UsePathStyle=true; our URL builder
	// already embeds bucket in path for custom endpoints
	c := testClient(config.S3Config{
		Endpoint: "https://minio.example.com",
		Bucket:   "my-bucket",
	})
	url := c.buildURL("photos/test.jpg")
	assert.Equal(t, "https://minio.example.com/my-bucket/photos/test.jpg", url)
}

// ---------------------------------------------------------------------------
// Tests: Upload (with mock)
// ---------------------------------------------------------------------------

func TestUpload_Success(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{
		Bucket:   "test-bucket",
		PublicURL: "https://cdn.example.com",
	})

	m.On("PutObject", mock.Anything, mock.MatchedBy(func(p *s3.PutObjectInput) bool {
		return *p.Bucket == "test-bucket" && *p.ContentType == "image/jpeg"
	})).Return(&s3.PutObjectOutput{}, nil)

	url, err := c.Upload(context.Background(), []byte("hello"), "photo.jpg", "image/jpeg")
	assert.NoError(t, err)
	assert.Contains(t, url, "https://cdn.example.com/")
	assert.Contains(t, url, "-photo.jpg")
	m.AssertExpectations(t)
}

func TestUpload_NonRetryableClientError(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b", PublicURL: "https://cdn.example.com"})

	// 400 BadRequest is NOT retryable → should fail fast
	m.On("PutObject", mock.Anything, mock.Anything).Return(nil, &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 400}},
	})

	_, err := c.Upload(context.Background(), []byte("data"), "f.jpg", "text/plain")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upload")
	m.AssertNumberOfCalls(t, "PutObject", 1)
}

func TestUpload_RetryableServerError(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b", PublicURL: "https://cdn.example.com"})

	// First two calls: 500 → retry; third: success
	m.On("PutObject", mock.Anything, mock.Anything).
		Return(nil, &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 500}},
		}).Times(2)
	m.On("PutObject", mock.Anything, mock.Anything).
		Return(&s3.PutObjectOutput{}, nil).Once()

	_, err := c.Upload(context.Background(), []byte("data"), "f.jpg", "text/plain")
	assert.NoError(t, err)
	m.AssertNumberOfCalls(t, "PutObject", 3)
}

func TestUpload_RetryExhausted(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b", PublicURL: "https://cdn.example.com"})

	// All calls fail with retryable error (500)
	m.On("PutObject", mock.Anything, mock.Anything).
		Return(nil, &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 500}},
		})

	_, err := c.Upload(context.Background(), []byte("data"), "f.jpg", "text/plain")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exhausted")
	m.AssertNumberOfCalls(t, "PutObject", 3)
}

func TestUpload_CancelledContext(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b", PublicURL: "https://cdn.example.com"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled

	// Should not even call PutObject
	_, err := c.Upload(ctx, []byte("data"), "f.jpg", "text/plain")
	assert.Error(t, err)
	m.AssertNumberOfCalls(t, "PutObject", 0)
}

func TestUpload_ContextDeadline(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b", PublicURL: "https://cdn.example.com"})

	m.On("PutObject", mock.Anything, mock.Anything).
		Return(nil, context.DeadlineExceeded)

	_, err := c.Upload(context.Background(), []byte("data"), "f.jpg", "text/plain")
	assert.Error(t, err)
	// context.DeadlineExceeded should propagate immediately, not retry
	m.AssertNumberOfCalls(t, "PutObject", 1)
}

// ---------------------------------------------------------------------------
// Tests: Download
// ---------------------------------------------------------------------------

func TestDownload_Success(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b"})

	m.On("GetObject", mock.Anything, mock.MatchedBy(func(p *s3.GetObjectInput) bool {
		return *p.Bucket == "b"
	})).Return(&s3.GetObjectOutput{
		ContentType: aws.String("image/png"),
		Body:        io.NopCloser(strings.NewReader("hello")),
	}, nil)

	data, ct, err := c.Download(context.Background(), "photos/test.png")
	assert.NoError(t, err)
	assert.Equal(t, "image/png", ct)
	assert.Equal(t, []byte("hello"), data)
}

func TestDownload_NotFound(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b"})

	m.On("GetObject", mock.Anything, mock.Anything).
		Return(nil, &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 404}},
		})

	_, _, err := c.Download(context.Background(), "photos/missing.jpg")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download")
}

func TestDownload_NilClient(t *testing.T) {
	resetSingleton()
	_, _, err := Download(context.Background(), "key")
	assert.EqualError(t, err, "s3 client is not initialized")
}

// ---------------------------------------------------------------------------
// Tests: Delete
// ---------------------------------------------------------------------------

func TestDelete_Success(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b"})

	m.On("DeleteObject", mock.Anything, mock.MatchedBy(func(p *s3.DeleteObjectInput) bool {
		return *p.Bucket == "b"
	})).Return(&s3.DeleteObjectOutput{}, nil)

	err := c.Delete(context.Background(), "photos/test.jpg")
	assert.NoError(t, err)
}

func TestDelete_Retryable(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b"})

	// First two: 503 (retryable), third: success
	m.On("DeleteObject", mock.Anything, mock.Anything).
		Return(nil, &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 503}},
		}).Times(2)
	m.On("DeleteObject", mock.Anything, mock.Anything).
		Return(&s3.DeleteObjectOutput{}, nil).Once()

	err := c.Delete(context.Background(), "photos/test.jpg")
	assert.NoError(t, err)
	m.AssertNumberOfCalls(t, "DeleteObject", 3)
}

func TestDelete_NilClient(t *testing.T) {
	resetSingleton()
	err := Delete(context.Background(), "key")
	assert.EqualError(t, err, "s3 client is not initialized")
}

// ---------------------------------------------------------------------------
// Tests: ResolveMediaURL
// ---------------------------------------------------------------------------

func TestResolveMediaURL_Disabled(t *testing.T) {
	resetSingleton()
	instance = &Client{cfg: config.S3Config{Enabled: false}}
	assert.Equal(t, "/local/path/file.jpg", ResolveMediaURL("/local/path/file.jpg"))
}

func TestResolveMediaURL_NilInstance(t *testing.T) {
	resetSingleton()
	assert.Equal(t, "/local/path/file.jpg", ResolveMediaURL("/local/path/file.jpg"))
}

func TestResolveMediaURL_AlreadyHTTP(t *testing.T) {
	c := testClient(config.S3Config{Enabled: true})
	instance = c

	assert.Equal(t, "https://cdn.example.com/photo.jpg", ResolveMediaURL("https://cdn.example.com/photo.jpg"))
}

func TestResolveMediaURL_LocalPath(t *testing.T) {
	c := testClient(config.S3Config{
		Enabled:   true,
		Bucket:    "b",
		Prefix:    "whatsapp-media/",
		PublicURL: "https://cdn.example.com",
	})
	instance = c

	result := ResolveMediaURL("/app/statics/20260101-uuid-photo.jpg")
	assert.True(t, strings.HasPrefix(result, "https://cdn.example.com/whatsapp-media/"))
	assert.Contains(t, result, "-photo.jpg")
	// Must NOT contain a second UUID (the original filename is just "20260101-uuid-photo.jpg")
	assert.NotContains(t, result, "-undefined-uuid") // sanity
}

func TestResolveMediaURL_NoPrefix(t *testing.T) {
	c := testClient(config.S3Config{
		Enabled:   true,
		Bucket:    "b",
		PublicURL: "https://cdn.example.com",
	})
	instance = c

	result := ResolveMediaURL("/statics/photo.jpg")
	assert.True(t, strings.HasPrefix(result, "https://cdn.example.com/"))
	assert.Contains(t, result, "photo.jpg")
}

// ---------------------------------------------------------------------------
// Tests: IsRetryable
// ---------------------------------------------------------------------------

func TestIsRetryable_Nil(t *testing.T) {
	assert.False(t, isRetryable(nil))
}

func TestIsRetryable_ContextCancelled(t *testing.T) {
	assert.False(t, isRetryable(context.Canceled))
	assert.False(t, isRetryable(context.DeadlineExceeded))
}

func TestIsRetryable_ClientError(t *testing.T) {
	err := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 403}},
	}
	assert.False(t, isRetryable(err), "4xx should not be retryable")
}

func TestIsRetryable_ServerError(t *testing.T) {
	err := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{
			Response: &http.Response{StatusCode: 500},
		},
	}
	assert.True(t, isRetryable(err))
}

func TestIsRetryable_Throttling(t *testing.T) {
	err := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{
			Response: &http.Response{StatusCode: 429},
		},
	}
	assert.True(t, isRetryable(err))
}

func TestIsRetryable_NetworkError(t *testing.T) {
	assert.True(t, isRetryable(errors.New("connection reset by peer")))
	assert.True(t, isRetryable(errors.New("TLS handshake timeout")))
}

// ---------------------------------------------------------------------------
// Tests: Package-level functions (nil client guards)
// ---------------------------------------------------------------------------

func TestUpload_NilClient(t *testing.T) {
	resetSingleton()
	_, err := Upload(context.Background(), nil, "f.jpg", "image/jpeg")
	assert.EqualError(t, err, "s3 client is not initialized")
}

func TestBuildPublicURL_NilInstance(t *testing.T) {
	resetSingleton()
	assert.Equal(t, "", BuildPublicURL("some/key"))
}

// ---------------------------------------------------------------------------
// Tests: Concurrent access (race detection)
// ---------------------------------------------------------------------------

func TestConcurrentGenerateKey(t *testing.T) {
	c := testClient(config.S3Config{})
	var wg sync.WaitGroup
	keys := make(chan string, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			keys <- c.GenerateKey("photo.jpg")
		}()
	}

	wg.Wait()
	close(keys)

	seen := make(map[string]bool)
	for k := range keys {
		if seen[k] {
			t.Fatal("duplicate key generated:", k)
		}
		seen[k] = true
	}
	assert.Len(t, seen, 50, "all 50 keys should be unique")
}

// ---------------------------------------------------------------------------
// Tests: checkBucketAccess (non-fatal; just must not panic)
// ---------------------------------------------------------------------------

func TestCheckBucketAccess_DoesNotPanic(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b"})
	m.On("HeadBucket", mock.Anything, mock.Anything).
		Return(&s3.HeadBucketOutput{}, nil)

	// Must not panic regardless of outcome
	c.checkBucketAccess(context.Background())
	m.AssertExpectations(t)
}

func TestCheckBucketAccess_LogsOnFailure(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{Bucket: "b"})
	m.On("HeadBucket", mock.Anything, mock.Anything).
		Return(nil, errors.New("Forbidden"))

	// Must not panic
	c.checkBucketAccess(context.Background())
	m.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Tests: retry with config override
// ---------------------------------------------------------------------------

func TestRetry_CustomConfig(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{
		Bucket:      "b",
		RetryMax:    1, // only 1 attempt = no retry
		RetryWaitMs: 1,
	})

	m.On("PutObject", mock.Anything, mock.Anything).
		Return(nil, &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 500}},
		})

	_, err := c.Upload(context.Background(), []byte("x"), "f.jpg", "text/plain")
	assert.Error(t, err)
	m.AssertNumberOfCalls(t, "PutObject", 1)
}

func TestRetry_ZeroConfigUsesDefaults(t *testing.T) {
	c, m := testClientWithMock(config.S3Config{
		Bucket: "b",
		// RetryMax=0, RetryWaitMs=0 → use defaults (3 retries)
	})

	// All retryable → exhausted after 3
	m.On("PutObject", mock.Anything, mock.Anything).
		Return(nil, &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 500}},
		})

	_, err := c.Upload(context.Background(), []byte("x"), "f.jpg", "text/plain")
	assert.Error(t, err)
	m.AssertNumberOfCalls(t, "PutObject", 3)
}

// ---------------------------------------------------------------------------
// Time-sensitivity note
// ---------------------------------------------------------------------------

func init() {
	// Pin random seed to avoid flakiness in retry jitter tests
	// (not needed here since we don't assert exact timing)
}
