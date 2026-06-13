package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// s3Client is the minimal interface for S3 operations we use.
// Extracted for testability — the real *s3.Client conforms to it.
type s3Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
}

var (
	instance *Client
	once     sync.Once
	mu       sync.Mutex

	// newClientFn is overridable in tests to avoid real AWS config loading.
	newClientFn = newClient
)

const (
	defaultRetryMax    = 3
	defaultRetryWaitMs = 250  // initial backoff
	uploadTimeout      = 5 * time.Minute
	deleteTimeout      = 30 * time.Second
	downloadTimeout    = 5 * time.Minute
	headTimeout        = 10 * time.Second
)

// Client wraps the AWS S3 SDK and its config.
type Client struct {
	s3Client s3Client
	cfg      config.S3Config
}

// Initialize creates the singleton S3 client from config.
// If S3 is disabled, it's a no-op.
func Initialize(cfg config.S3Config) error {
	if !cfg.Enabled {
		logrus.Info("S3 storage is disabled, skipping initialization")
		return nil
	}

	if cfg.Bucket == "" {
		return fmt.Errorf("s3 bucket is required when S3 is enabled")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return fmt.Errorf("s3 access key and secret key are required when S3 is enabled")
	}

	var err error
	once.Do(func() {
		instance, err = newClientFn(cfg)
	})
	if err != nil {
		once = sync.Once{}
		return err
	}

	// Best-effort bucket reachability check (non-fatal, logs only).
	instance.checkBucketAccess(context.Background())

	logrus.Infof("S3 storage initialized: bucket=%s, region=%s, endpoint=%s", cfg.Bucket, cfg.Region, cfg.Endpoint)
	return nil
}

// Reinitialize resets the singleton S3 client and reinitializes with new config.
// This is used for runtime configuration changes without requiring a server restart.
// Thread-safe: uses mutex to prevent race conditions during reinitialization.
func Reinitialize(cfg config.S3Config) error {
	mu.Lock()
	defer mu.Unlock()

	// Reset singleton
	instance = nil
	once = sync.Once{}

	// Reinitialize with new config
	return Initialize(cfg)
}

// TestConnection tests an S3 configuration without affecting the singleton.
// It creates a temporary client and checks bucket access.
func TestConnection(ctx context.Context, cfg config.S3Config) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Bucket == "" {
		return fmt.Errorf("s3 bucket is required when S3 is enabled")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return fmt.Errorf("s3 access key and secret key are required when S3 is enabled")
	}

	client, err := newClientFn(cfg)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, headTimeout)
	defer cancel()

	_, err = client.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to access bucket %q: %w", cfg.Bucket, err)
	}

	return nil
}

func newClient(cfg config.S3Config) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	realClient := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})

	return &Client{
		s3Client: realClient,
		cfg:      cfg,
	}, nil
}

// checkBucketAccess verifies the bucket exists and is reachable.
// Non-fatal: only logs a warning so the app can still serve degraded.
func (c *Client) checkBucketAccess(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, headTimeout)
	defer cancel()

	_, err := c.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.cfg.Bucket),
	})
	if err != nil {
		logrus.Warnf("S3 bucket %q reachability check failed (will retry on first upload): %v", c.cfg.Bucket, err)
	} else {
		logrus.Debugf("S3 bucket %q reachability check passed", c.cfg.Bucket)
	}
}

// IsEnabled returns true when S3 was successfully initialized.
func IsEnabled() bool {
	return instance != nil && instance.cfg.Enabled
}

// GetClient returns the singleton S3 client (nil if uninitialized).
func GetClient() *Client {
	return instance
}

// ---------------------------------------------------------------------------
// High-level package functions (delegate to singleton)
// ---------------------------------------------------------------------------

// Upload uploads data to S3 and returns the public URL. Retries on transient errors.
func Upload(ctx context.Context, data []byte, filename string, contentType string) (string, error) {
	if instance == nil {
		return "", fmt.Errorf("s3 client is not initialized")
	}
	return instance.Upload(ctx, data, filename, contentType)
}

// Download retrieves an object from S3 by key.
func Download(ctx context.Context, key string) ([]byte, string, error) {
	if instance == nil {
		return nil, "", fmt.Errorf("s3 client is not initialized")
	}
	return instance.Download(ctx, key)
}

// Delete removes an object from S3 by key. Retries on transient errors.
func Delete(ctx context.Context, key string) error {
	if instance == nil {
		return fmt.Errorf("s3 client is not initialized")
	}
	return instance.Delete(ctx, key)
}

// BuildPublicURL builds the object URL from a key using the configured URL scheme.
func BuildPublicURL(key string) string {
	if instance == nil {
		return ""
	}
	return instance.buildURL(key)
}

// ResolveMediaURL attempts to convert a stored media path to an accessible S3 URL.
//   - If S3 is disabled, the original path is returned unchanged.
//   - If the path is already an HTTP(S) URL (e.g. already resolved), it is returned as-is.
//   - For local paths, a deterministic S3 key is constructed from the filename
//     (without adding a new random UUID — preserves the original filename identity).
func ResolveMediaURL(mediaPath string) string {
	if instance == nil || !instance.cfg.Enabled {
		return mediaPath
	}
	if strings.HasPrefix(mediaPath, "http://") || strings.HasPrefix(mediaPath, "https://") {
		return mediaPath
	}

	// Build a deterministic key from the filename so repeated calls
	// always point to the same virtual path.  The caller is responsible
	// for ensuring the file was actually uploaded there.
	filename := path.Base(mediaPath)
	timestamp := time.Now().Format("20060102")
	prefix := strings.TrimSuffix(instance.cfg.Prefix, "/")

	var key string
	if prefix != "" {
		key = fmt.Sprintf("%s/%s/%s", prefix, timestamp, filename)
	} else {
		key = fmt.Sprintf("%s/%s", timestamp, filename)
	}
	return instance.buildURL(key)
}

// ---------------------------------------------------------------------------
// Instance methods
// ---------------------------------------------------------------------------

// Upload uploads data to S3. Retries on transient failures.
func (c *Client) Upload(ctx context.Context, data []byte, filename string, contentType string) (string, error) {
	key := c.GenerateKey(filename)

	uploadFn := func(ctx context.Context) error {
		_, err := c.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(c.cfg.Bucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(data),
			ContentType: aws.String(contentType),
		})
		return err
	}

	if err := c.retry(ctx, uploadFn); err != nil {
		return "", fmt.Errorf("failed to upload to S3 after retries: %w", err)
	}

	url := c.buildURL(key)
	logrus.Debugf("Uploaded media to S3: %s", url)
	return url, nil
}

// Download retrieves an object from S3. Returns (data, contentType, error).
func (c *Client) Download(ctx context.Context, key string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	resp, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to download from S3: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read S3 response body: %w", err)
	}

	contentType := ""
	if resp.ContentType != nil {
		contentType = *resp.ContentType
	}
	return data, contentType, nil
}

// Delete removes an object from S3. Retries on transient failures.
func (c *Client) Delete(ctx context.Context, key string) error {
	deleteFn := func(ctx context.Context) error {
		_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(c.cfg.Bucket),
			Key:    aws.String(key),
		})
		return err
	}

	if err := c.retry(ctx, deleteFn); err != nil {
		return fmt.Errorf("failed to delete from S3 after retries: %w", err)
	}

	logrus.Debugf("Deleted S3 object: %s", key)
	return nil
}

// GenerateKey creates a unique object key with optional prefix, date-based
// partitioning, a random UUID, and the original filename.
func (c *Client) GenerateKey(filename string) string {
	timestamp := time.Now().Format("20060102")
	uniqueID := uuid.NewString()

	prefix := strings.TrimSuffix(c.cfg.Prefix, "/")
	if prefix != "" {
		return fmt.Sprintf("%s/%s/%s-%s", prefix, timestamp, uniqueID, filename)
	}
	return fmt.Sprintf("%s/%s-%s", timestamp, uniqueID, filename)
}

// buildURL returns the publicly accessible URL for the given key.
// Priority: configured PublicURL > custom endpoint > AWS default virtual-hosted.
func (c *Client) buildURL(key string) string {
	if c.cfg.PublicURL != "" {
		base := strings.TrimSuffix(c.cfg.PublicURL, "/")
		return base + "/" + key
	}

	if c.cfg.Endpoint != "" {
		endpoint := strings.TrimSuffix(c.cfg.Endpoint, "/")
		return fmt.Sprintf("%s/%s/%s", endpoint, c.cfg.Bucket, key)
	}

	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", c.cfg.Bucket, c.cfg.Region, key)
}

// ---------------------------------------------------------------------------
// Retry
// ---------------------------------------------------------------------------

// retry executes fn with exponential backoff up to defaultRetryMax attempts.
// Context cancellation is NOT retried — it propagates immediately.
func (c *Client) retry(ctx context.Context, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()

	var lastErr error
	maxRetries := defaultRetryMax
	if c.cfg.RetryMax > 0 {
		maxRetries = c.cfg.RetryMax
	}

	waitMs := defaultRetryWaitMs
	if c.cfg.RetryWaitMs > 0 {
		waitMs = c.cfg.RetryWaitMs
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check context before every attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if attempt > 0 {
			// Exponential backoff with jitter
			backoff := time.Duration(waitMs) * time.Millisecond * time.Duration(math.Pow(2, float64(attempt-1)))
			jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff + jitter):
			}
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err
		if !isRetryable(err) {
			return err
		}

		logrus.Debugf("S3 operation attempt %d/%d failed (retryable): %v", attempt+1, maxRetries, err)
	}

	return fmt.Errorf("exhausted %d retries: %w", maxRetries, lastErr)
}

// isRetryable reports whether the error is worth retrying.
// Context-cancellation and 4xx client errors (except 429) are not retried.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Never retry deliberate cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check HTTP response status
	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) {
		code := responseErr.Response.StatusCode
		// 5xx = server fault, 429 = throttling → retry
		if code >= 500 || code == 429 {
			return true
		}
		// 4xx client errors are NOT retryable
		return false
	}

	// Network / connection / I/O timeouts → retry
	return true
}
