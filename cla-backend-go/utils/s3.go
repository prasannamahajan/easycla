package utils

import (
	"bytes"
	"errors"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// PresignedURLValidity is time for which s3 url will remain valid
const PresignedURLValidity = 15 * time.Minute

// S3Storage provides methods to handle s3 storage
type S3Storage interface {
	Upload(fileContent []byte, projectID string, claType string, identifier string, signatureID string) error
	Download(filename string) ([]byte, error)
	Delete(filename string) error
	GetPresignedURL(filename string) (string, error)
}

var s3Storage S3Storage

// S3Client struct provide methods to interact with s3
type S3Client struct {
	s3         *s3.S3
	BucketName string
}

// SetS3Storage set default S3Storage
func SetS3Storage(awsSession *session.Session, bucketName string) {
	s3Storage = &S3Client{
		s3:         s3.New(awsSession),
		BucketName: bucketName,
	}
}

// Upload file to s3 storage at path contract-group/<project-ID>/<claType>/<identifier>/<signatureID>.pdf
// claType should be cla or ccla
// identifier can be user-id or company-id
func (s3c *S3Client) Upload(fileContent []byte, projectID string, claType string, identifier string, signatureID string) error {
	filename := strings.Join([]string{"contract-group", projectID, claType, identifier, signatureID}, "/") + ".pdf"
	_, err := s3c.s3.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s3c.BucketName),
		Key:    aws.String(filename),
		Body:   bytes.NewReader(fileContent),
	})
	return err
}

// Download file from s3
func (s3c *S3Client) Download(filename string) ([]byte, error) {
	ou, err := s3c.s3.GetObject(&s3.GetObjectInput{
		Bucket: &s3c.BucketName,
		Key:    &filename,
	})
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(ou.Body)
	if err != nil {
		return nil, err
	}
	return body, err
}

// Delete file from s3
func (s3c *S3Client) Delete(filename string) error {
	_, err := s3c.s3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &s3c.BucketName,
		Key:    &filename,
	})
	return err
}

// GetPresignedURL provided presigned url for download
func (s3c *S3Client) GetPresignedURL(filename string) (string, error) {
	req, _ := s3c.s3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &s3c.BucketName,
		Key:    &filename,
	})
	url, err := req.Presign(PresignedURLValidity)
	if err != nil {
		return "", err
	}
	return url, nil
}

// UploadToS3 uploads file to s3 storage at path contract-group/<project-ID>/<claType>/<identifier>/<signatureID>.pdf
// claType should be cla or ccla
// identifier can be user-id or company-id
func UploadToS3(body []byte, projectID string, claType string, identifier string, signatureID string) error {
	if s3Storage == nil {
		return errors.New("s3Storage not set")
	}
	return s3Storage.Upload(body, projectID, claType, identifier, signatureID)
}

// DownloadFromS3 downloads file from s3
func DownloadFromS3(filename string) ([]byte, error) {
	if s3Storage == nil {
		return nil, errors.New("s3Storage not set")
	}
	return s3Storage.Download(filename)
}

// DeleteFromS3 deletes file from s3
func DeleteFromS3(filename string) error {
	if s3Storage == nil {
		return errors.New("s3Storage not set")
	}
	return s3Storage.Delete(filename)
}

// GetDownloadLink provides presigned s3 url
func GetDownloadLink(filename string) (string, error) {
	if s3Storage == nil {
		return "", errors.New("s3Storage not set")
	}
	return s3Storage.GetPresignedURL(filename)
}

// SignedCLAFilename provide s3 bucket url
func SignedCLAFilename(projectID string, claType string, identifier string, signatureID string) string {
	return strings.Join([]string{"contract-group", projectID, claType, identifier, signatureID}, "/") + ".pdf"
}
