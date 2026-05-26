package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Config holds the connection + bucket settings for the S3-compatible
// storage backend. The defaults target the in-cluster SeaweedFS filer's
// S3 API (compose service seaweedfs-filer with the -s3 flag); for real
// AWS S3, MinIO, R2, etc. swap the Endpoint and credentials.
type S3Config struct {
	Endpoint        string // e.g. http://seaweedfs-filer:8333
	Region          string // any value works for SeaweedFS; required by the SDK
	Bucket          string // e.g. raven-docs
	AccessKeyID     string // empty for SeaweedFS anonymous (default)
	SecretAccessKey string // empty for SeaweedFS anonymous (default)
	// UsePathStyle is mandatory for SeaweedFS / MinIO; AWS S3 takes
	// virtual-hosted style when set to false. Defaults to true so the
	// zero-config in-cluster path Just Works.
	UsePathStyle bool
}

// S3Client is an S3-compatible storage backend that uses the official AWS
// Go SDK. Replaces the hand-rolled SeaweedFS HTTP client which got a class
// of multipart-envelope bugs wrong (storing the entire form-data wrapper
// alongside the file body). The SDK handles the wire protocol, retries,
// content-MD5 checks, and large-file multipart uploads.
type S3Client struct {
	c      *s3.Client
	bucket string
}

// NewS3Client constructs an S3Client and verifies the target bucket exists
// (creating it on first run, since SeaweedFS doesn't auto-create). Returns
// a typed error wrapping the SDK so callers can distinguish config
// problems from runtime failures.
func NewS3Client(ctx context.Context, cfg S3Config) (*S3Client, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("storage/s3: Endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("storage/s3: Bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1" // SDK default; SeaweedFS ignores
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKeyID != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	} else {
		// SeaweedFS S3 accepts anonymous by default; the SDK still wants
		// non-empty credentials to sign the request, so plug in a fixed
		// stub. AnonymousCredentials would skip signing entirely, which
		// some S3 implementations reject.
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("anonymous", "anonymous", ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("storage/s3: load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.UsePathStyle || true
	})

	c := &S3Client{c: client, bucket: cfg.Bucket}
	if err := c.ensureBucket(ctx); err != nil {
		return nil, fmt.Errorf("storage/s3: ensure bucket %q: %w", cfg.Bucket, err)
	}
	return c, nil
}

// ensureBucket is a no-op on AWS (the bucket must already exist and be
// owned by us) but creates the bucket on SeaweedFS / MinIO when missing,
// since those backends rely on bootstrap-time creation rather than IAM.
// Existing buckets surface as BucketAlreadyOwnedByYou / BucketAlreadyExists,
// both of which we treat as success.
func (c *S3Client) ensureBucket(ctx context.Context) error {
	_, err := c.c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(c.bucket)})
	if err == nil {
		return nil
	}
	// Try create. SeaweedFS doesn't return NotFound consistently so we
	// always attempt creation and squash already-exists errors.
	_, cerr := c.c.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(c.bucket)})
	if cerr == nil {
		return nil
	}
	var apiErr smithy.APIError
	if errors.As(cerr, &apiErr) {
		switch apiErr.ErrorCode() {
		case "BucketAlreadyOwnedByYou", "BucketAlreadyExists":
			return nil
		}
	}
	return cerr
}

// Upload streams the file into the bucket under a content-addressed key and
// returns the storage path (the S3 object key, prefixed with the bucket so
// the rest of the codebase can keep one opaque string in documents.storage_path).
//
// The key is a random 32-char hex slug + the original extension; the
// `filename` is preserved in metadata for downloads that need it. We do
// NOT use the original filename as the key — duplicate names would collide,
// and arbitrary user filenames let bad input punch through into the URL
// space (Unicode, path traversal, etc).
func (c *S3Client) Upload(ctx context.Context, filename string, reader io.Reader) (string, error) {
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("storage/s3: generate key: %w", err)
	}
	key := hex.EncodeToString(keyBytes) + extensionFor(filename)

	_, err := c.c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentTypeFor(filename)),
		Metadata: map[string]string{
			"original-filename": filename,
		},
	})
	if err != nil {
		return "", fmt.Errorf("storage/s3: put object: %w", err)
	}
	return c.bucket + "/" + key, nil
}

// Download fetches the object body for a given storage path. Storage paths
// produced by Upload are of the form "bucket/key" so the bucket lives in
// the row alongside the key — that way reading old rows after a bucket
// rename still finds the bytes via the embedded bucket name.
//
// Backwards compatibility note: pre-S3 rows stored a SeaweedFS fid like
// "1,04f392dbde". Those rows are NOT readable by this client and need to be
// re-uploaded; the demo box's three test docs will be wiped during the cut-
// over migration.
func (c *S3Client) Download(ctx context.Context, storagePath string) (io.ReadCloser, error) {
	bucket, key := splitStoragePath(storagePath, c.bucket)
	out, err := c.c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("storage/s3: get object %s: %w", storagePath, err)
	}
	return out.Body, nil
}

// Delete removes the object identified by the storage path. NoSuchKey is
// treated as success — the caller's intent (the object is gone) is
// satisfied either way.
func (c *S3Client) Delete(ctx context.Context, storagePath string) error {
	bucket, key := splitStoragePath(storagePath, c.bucket)
	_, err := c.c.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *s3types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil
		}
		// Some implementations return a generic 404 instead of typed NoSuchKey.
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey" {
			return nil
		}
		var responseErr interface{ HTTPStatusCode() int }
		if errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("storage/s3: delete %s: %w", storagePath, err)
	}
	return nil
}

// splitStoragePath separates "bucket/key" produced by Upload, falling back
// to (defaultBucket, raw) when no slash is present (covers legacy callers
// that still pass a bare key).
func splitStoragePath(storagePath, defaultBucket string) (bucket, key string) {
	if i := strings.IndexByte(storagePath, '/'); i > 0 {
		return storagePath[:i], storagePath[i+1:]
	}
	return defaultBucket, storagePath
}

func extensionFor(filename string) string {
	ext := path.Ext(filename)
	if len(ext) > 16 {
		// Avoid pathological "files" with absurdly long pseudo-extensions
		// blowing up the key length.
		return ""
	}
	return strings.ToLower(ext)
}

func contentTypeFor(filename string) string {
	switch strings.ToLower(path.Ext(filename)) {
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}
