package config

// S3Config holds the configuration for S3-compatible object storage.
// Supports AWS S3, Cloudflare R2, MinIO, and other S3-compatible services.
type S3Config struct {
	Enabled   bool
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Endpoint  string // Custom endpoint URL for R2, MinIO, etc.
	Prefix    string // Object key prefix, e.g. "whatsapp-media/"
	PublicURL string // Optional public URL base for generating accessible URLs

	// Retry settings for transient S3 failures.
	// Zero values use sensible defaults (3 retries, 250ms initial backoff).
	RetryMax    int `json:"retry_max"`    // max retry attempts (default: 3)
	RetryWaitMs int `json:"retry_wait_ms"` // initial backoff in ms (default: 250)
}
