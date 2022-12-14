package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mushroomsir/mimetypes"
	"github.com/sirupsen/logrus"
)

type fileWalk chan string

func (f fileWalk) Walk(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if !info.IsDir() {
		f <- path
	}
	return nil
}

func main() {
	debug := flag.Bool("debug", false, "Debug mode")
	endpointURL := flag.String("endpointurl", "https://storage.yandexcloud.net", "Endpoint URL")
	signingRegion := flag.String("region", "ru-central1", "Region name")
	bucketName := flag.String("bucket", "", "Bucket name")
	akid := flag.String("akid", "", "aws_access_key_id")
	asak := flag.String("asak", "", "aws_secret_access_key")
	uploadDir := flag.String("uploaddir", "", "Dir path to upload")

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

	walker := make(fileWalk)
	go func() {
		// Gather the files to upload by walking the path recursively
		if err := filepath.Walk(*uploadDir, walker.Walk); err != nil {
			logrus.Fatalln("Walk failed:", err)
		}
		close(walker)
	}()

	// Create custom endpoint resolver for returning correct URL for S3 storage in ru-central1 region
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:   "yc",
			URL:           *endpointURL,
			SigningRegion: *signingRegion,
		}, nil
	})

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(*akid, *asak, "")),
	)
	if err != nil {
		logrus.Fatalf("Failed to create a config: %v", err)
	}

	// For each file found walking, upload it to Amazon S3
	uploader := manager.NewUploader(s3.NewFromConfig(cfg))
	for path := range walker {
		rel, err := filepath.Rel(*uploadDir, path)
		if err != nil {
			logrus.Fatalln("Unable to get relative path:", path, err)
		}

		file, err := os.Open(path)
		if err != nil {
			logrus.Println("Failed opening file", path, err)
			continue
		}
		defer file.Close()

		mtype := mimetypes.Lookup(path)
		result, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
			Bucket:      bucketName,
			Key:         aws.String(filepath.Join("./", rel)),
			Body:        file,
			ContentType: stringPtr(mtype),
		})
		if err != nil {
			logrus.Errorf("Failed to upload %q: %v", path, err)
			continue
		}
		logrus.Debugf("Uploaded: %q, %s, %s", path, result.Location, mtype)
	}

	logrus.Infof("Successfully uploaded directory: %q to bucket: %q", *uploadDir, *bucketName)
}

func stringPtr(s string) *string {
	return &s
}
