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
}
