package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
	"github.com/AdityaTaggar05/styx/internal/store"
)

// S3 implements store.DataStore for Amazon S3 and S3-compatible services
// (MinIO, Cloudflare R2, Backblaze B2) via a configurable endpoint.
type S3 struct {
	bucket       string
	region       string
	endpoint     string
	accessKey    string
	secretKey    string
	sessionToken string

	client *awss3.Client
}

var _ store.DataStore = (*S3)(nil)

// [CONSTRUCTION]

// New creates an S3 backend from a config map.
// Required: bucket. Optional: region (default us-east-1), endpoint,
// access_key_id, secret_access_key, session_token.
func New(cfg map[string]string) (*S3, error) {
	bucket := cfg["bucket"]
	if bucket == "" {
		return nil, fmt.Errorf("%w: s3 requires a bucket", styxErrors.ErrConfigValidate)
	}

	region := cfg["region"]
	if region == "" {
		region = "us-east-1"
	}

	return &S3{
		bucket:       bucket,
		region:       region,
		endpoint:     cfg["endpoint"],
		accessKey:    cfg["access_key_id"],
		secretKey:    cfg["secret_access_key"],
		sessionToken: cfg["session_token"],
	}, nil
}

func init() {
	store.Register("s3", func(cfg map[string]string) (store.DataStore, error) {
		return New(cfg)
	})
}

func (s *S3) Name() string { return "s3" }

// [CLIENT]

func (s *S3) getClient(ctx context.Context) (*awss3.Client, error) {
	if s.client != nil {
		return s.client, nil
	}

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(s.region),
	}
	if s.accessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(s.accessKey, s.secretKey, s.sessionToken),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrAuthRequired, err)
	}

	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if s.endpoint != "" {
			o.BaseEndpoint = aws.String(s.endpoint)
			o.UsePathStyle = true
		}
	})

	s.client = client
	return client, nil
}

// [AUTH]

// AuthURL is empty: S3 uses access keys, not a browser OAuth flow.
func (s *S3) AuthURL() string { return "" }

// Authenticate validates that the configured credentials can reach the bucket.
func (s *S3) Authenticate(ctx context.Context, code string) error {
	client, err := s.getClient(ctx)
	if err != nil {
		return err
	}

	_, err = client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	if isNotFound(err) {
		return fmt.Errorf("%w: bucket %q", styxErrors.ErrStoreNotFound, s.bucket)
	}
	return fmt.Errorf("%w: %v", styxErrors.ErrAuthInvalid, err)
}

// IsAuthenticated reports whether the bucket is reachable with current creds.
func (s *S3) IsAuthenticated(ctx context.Context) (bool, error) {
	client, err := s.getClient(ctx)
	if err != nil {
		return false, nil
	}

	_, err = client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Logout is a no-op: S3 credentials live in the config / AWS credential chain.
func (s *S3) Logout(ctx context.Context) error { return nil }

// [FILE CRUD]

func (s *S3) Pull(ctx context.Context, remotePath, localPath string) error {
	client, err := s.getClient(ctx)
	if err != nil {
		return err
	}

	out, err := client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey(remotePath)),
	})
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: %s", styxErrors.ErrStoreNotFound, remotePath)
		}
		if isAccessDenied(err) {
			return fmt.Errorf("%w: %s", styxErrors.ErrStorePermission, remotePath)
		}
		return fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
	}
	defer out.Body.Close()

	if dir := filepath.Dir(localPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
		}
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	return nil
}

func (s *S3) Push(ctx context.Context, localPath, remotePath string) (*store.FileMeta, error) {
	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", styxErrors.ErrFileNotFound, localPath)
		}
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrFilePermission, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	key := objectKey(remotePath)
	out, err := client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		if isAccessDenied(err) {
			return nil, fmt.Errorf("%w: %s", styxErrors.ErrStorePermission, remotePath)
		}
		return nil, fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
	}

	return &store.FileMeta{
		Path:     remotePath,
		Hash:     hashFromETag(out.ETag),
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		RemoteID: key,
	}, nil
}

func (s *S3) Delete(ctx context.Context, remotePath string) error {
	client, err := s.getClient(ctx)
	if err != nil {
		return err
	}

	// Deleting a missing key is a no-op in S3, so this is idempotent.
	_, err = client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey(remotePath)),
	})
	if err != nil {
		if isAccessDenied(err) {
			return fmt.Errorf("%w: %s", styxErrors.ErrStorePermission, remotePath)
		}
		return fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
	}

	return nil
}

// [METADATA]

func (s *S3) Hash(ctx context.Context, remotePath string) (string, error) {
	client, err := s.getClient(ctx)
	if err != nil {
		return "", err
	}

	out, err := client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey(remotePath)),
	})
	if err != nil {
		if isNotFound(err) {
			return "", fmt.Errorf("%w: %s", styxErrors.ErrStoreNotFound, remotePath)
		}
		return "", fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
	}

	return hashFromETag(out.ETag), nil
}

func (s *S3) List(ctx context.Context, remotePath string) ([]store.FileMeta, error) {
	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	paginator := awss3.NewListObjectsV2Paginator(client, &awss3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(prefixFor(remotePath)),
		Delimiter: aws.String("/"),
	})

	var result []store.FileMeta
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
		}

		for _, cp := range page.CommonPrefixes {
			if cp.Prefix == nil {
				continue
			}
			dirKey := strings.TrimSuffix(*cp.Prefix, "/")
			result = append(result, store.FileMeta{
				Path:     "/" + dirKey,
				IsDir:    true,
				RemoteID: dirKey,
			})
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			result = append(result, store.FileMeta{
				Path:     "/" + *obj.Key,
				Hash:     hashFromETag(obj.ETag),
				Size:     aws.ToInt64(obj.Size),
				ModTime:  aws.ToTime(obj.LastModified),
				RemoteID: *obj.Key,
			})
		}
	}

	return result, nil
}

func (s *S3) ListRecursive(ctx context.Context, remotePath string) ([]store.FileMeta, error) {
	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	paginator := awss3.NewListObjectsV2Paginator(client, &awss3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefixFor(remotePath)),
	})

	var result []store.FileMeta
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", styxErrors.ErrStoreUnavailable, err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			key := *obj.Key
			if strings.HasSuffix(key, "/") {
				continue // zero-byte "directory" marker object
			}
			result = append(result, store.FileMeta{
				Path:     "/" + key,
				Hash:     hashFromETag(obj.ETag),
				Size:     aws.ToInt64(obj.Size),
				ModTime:  aws.ToTime(obj.LastModified),
				RemoteID: key,
			})
		}
	}

	return result, nil
}

// [CHANGE DETECTION]

// ChangesSince is unsupported: S3 has no change feed, so the daemon relies on
// its periodic full sync to observe remote changes.
func (s *S3) ChangesSince(ctx context.Context, cursor string) ([]store.Change, string, error) {
	return nil, cursor, nil
}

// [HELPERS]

// objectKey maps a remote path like "/styx/projects/a.txt" to the S3 key
// "styx/projects/a.txt" (S3 keys have no leading slash).
func objectKey(remotePath string) string {
	return strings.TrimPrefix(remotePath, "/")
}

// prefixFor returns the key prefix for listing a remote directory.
func prefixFor(remotePath string) string {
	key := objectKey(remotePath)
	if key == "" {
		return ""
	}
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}
	return key
}

// hashFromETag converts an S3 ETag into the store's "md5:<hex>" form.
// For single-part uploads the ETag is the MD5 of the object.
func hashFromETag(etag *string) string {
	if etag == nil {
		return ""
	}
	return "md5:" + strings.Trim(*etag, `"`)
}

func apiErrorCode(err error) string {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		return ae.ErrorCode()
	}
	return ""
}

func isNotFound(err error) bool {
	switch apiErrorCode(err) {
	case "NotFound", "NoSuchKey", "NoSuchBucket", "404":
		return true
	}
	return false
}

func isAccessDenied(err error) bool {
	switch apiErrorCode(err) {
	case "AccessDenied", "Forbidden", "403":
		return true
	}
	return false
}
