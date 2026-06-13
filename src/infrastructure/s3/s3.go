package s3

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	instance *Client
	once     sync.Once
)

type Client struct {
	s3Client *s3.Client
	cfg      config.S3Config
}

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
		instance, err = newClient(cfg)
	})
	if err != nil {
		once = sync.Once{}
		return err
	}

	logrus.Infof("S3 storage initialized: bucket=%s, region=%s, endpoint=%s", cfg.Bucket, cfg.Region, cfg.Endpoint)
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

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})

	return &Client{
		s3Client: s3Client,
		cfg:      cfg,
	}, nil
}

func GetClient() *Client {
	return instance
}

func IsEnabled() bool {
	return instance != nil && instance.cfg.Enabled
}

func (c *Client) Upload(ctx context.Context, data []byte, filename string, contentType string) (string, error) {
	key := c.GenerateKey(filename)

	_, err := c.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	url := c.buildURL(key)
	logrus.Debugf("Uploaded media to S3: %s", url)
	return url, nil
}

func (c *Client) GenerateKey(filename string) string {
	timestamp := time.Now().Format("20060102")
	uniqueID := uuid.NewString()

	prefix := strings.TrimSuffix(c.cfg.Prefix, "/")
	if prefix != "" {
		return fmt.Sprintf("%s/%s/%s-%s", prefix, timestamp, uniqueID, filename)
	}
	return fmt.Sprintf("%s/%s-%s", timestamp, uniqueID, filename)
}

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

func Upload(ctx context.Context, data []byte, filename string, contentType string) (string, error) {
	if instance == nil {
		return "", fmt.Errorf("S3 client is not initialized")
	}
	return instance.Upload(ctx, data, filename, contentType)
}

func BuildPublicURL(key string) string {
	if instance == nil {
		return ""
	}
	return instance.buildURL(key)
}

func ResolveMediaURL(mediaPath string) string {
	if !IsEnabled() {
		return mediaPath
	}
	if strings.HasPrefix(mediaPath, "http://") || strings.HasPrefix(mediaPath, "https://") {
		return mediaPath
	}
	filename := path.Base(mediaPath)
	if instance == nil {
		return mediaPath
	}
	return instance.buildURL(instance.GenerateKey(filename))
}
