package s3

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// S3Client handles s3 uploads.
type S3Client struct {
	s3        *s3manager.Uploader
	bucket    *string
	deleter   *s3.S3
	aclSetter *s3.S3
}

// NewS3Uploader returns an s3 uploader for the given bucket.
func NewS3Uploader(bucket string, sess *session.Session) *S3Client {
	s3 := s3.New(sess)
	return &S3Client{
		s3:        s3manager.NewUploader(sess),
		bucket:    aws.String(bucket),
		deleter:   s3,
		aclSetter: s3,
	}
}

// Upload takes an io.Reader and a key, uploads the file at the reader to the key,
// and returns the s3 upload output from the Uploader.
func (s *S3Client) PublicUpload(file io.Reader, key string) (*s3manager.UploadOutput, error) {
	result, err := s.s3.Upload(&s3manager.UploadInput{
		Bucket: s.bucket,
		Key:    aws.String(key),
		Body:   file,
		ACL:    aws.String("public-read"),
	})
	if err != nil {
		return nil, err
	}
	_, err = s.aclSetter.PutObjectAcl(&s3.PutObjectAclInput{
		Bucket: s.bucket,
		Key:    aws.String(key),
		ACL:    aws.String("public-read"),
	})
	if err != nil {
		return nil, fmt.Errorf("set object acl: %w", err)
	}

	return result, nil
}

func (s *S3Client) Delete(key string) (*s3.DeleteObjectOutput, error) {
	result, err := s.deleter.DeleteObject(
		&s3.DeleteObjectInput{
			Bucket: s.bucket,
			Key:    aws.String(key),
		},
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}
