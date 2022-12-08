// snippet-sourcetype:[full-example]
package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/sirupsen/logrus"
)

var dirPrefixToStrinp string

func main() {
	debug := flag.Bool("debug", false, "Debug mode")
	endpointURL := flag.String("endpointurl", "https://storage.yandexcloud.net", "Endpoint URL")
	bucketName := flag.String("bucket", "", "Bucket name")
	uploadDir := flag.String("uploaddir", "", "Dir path to upload")
	akid := flag.String("akid", "", "aws_access_key_id")
	asak := flag.String("asak", "", "aws_secret_access_key")
	region := flag.String("region", "ru-central1", "Region name")

	flag.Parse()

	if *bucketName == "" || *uploadDir == "" {
		logrus.Fatal("You must supply the name of a bucket and upload dir")
	}

	if *akid == "" || *asak == "" {
		logrus.Fatal("You must supply the aws_access_key_id and aws_secret_access_key")
	}

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	dirPrefixToStrinp = *uploadDir
	di := NewDirectoryIterator(*bucketName, *uploadDir)

	sess, err := session.NewSession(&aws.Config{
		Endpoint:    endpointURL,
		Credentials: credentials.NewStaticCredentials(*akid, *asak, ""),
		Region:      aws.String(*region),
	})

	if err != nil {
		logrus.Fatalf("Failed to create a session: %v", err)
	}

	uploader := s3manager.NewUploader(sess)

	if err := uploader.UploadWithIterator(aws.BackgroundContext(), di); err != nil {
		logrus.Fatalf("Failed to upload: %v", err)
	}
	logrus.Infof("Successfully uploaded %q to %q", *uploadDir, *bucketName)
}

// DirectoryIterator represents an iterator of a specified directory
type DirectoryIterator struct {
	filePaths []string
	bucket    string
	next      struct {
		path string
		f    *os.File
	}
	err error
}

// NewDirectoryIterator builds a new DirectoryIterator
func NewDirectoryIterator(bucket, dir string) s3manager.BatchUploadIterator {
	var paths []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})

	return &DirectoryIterator{
		filePaths: paths,
		bucket:    bucket,
	}
}

// Next returns whether next file exists or not
func (di *DirectoryIterator) Next() bool {
	if len(di.filePaths) == 0 {
		di.next.f = nil
		return false
	}

	f, err := os.Open(di.filePaths[0])
	di.err = err
	di.next.f = f
	di.next.path = strings.TrimPrefix(di.filePaths[0], dirPrefixToStrinp)
	di.filePaths = di.filePaths[1:]

	return true && di.Err() == nil
}

// Err returns error of DirectoryIterator
func (di *DirectoryIterator) Err() error {
	return di.err
}

// UploadObject uploads a file
func (di *DirectoryIterator) UploadObject() s3manager.BatchUploadObject {
	f := di.next.f
	return s3manager.BatchUploadObject{
		Object: &s3manager.UploadInput{
			Bucket: &di.bucket,
			Key:    &di.next.path,
			Body:   f,
		},
		After: func() error {
			return f.Close()
		},
	}
}
