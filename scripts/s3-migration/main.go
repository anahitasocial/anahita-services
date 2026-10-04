package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const newPathPrefix = "assets/v2/"

// bucketName is read from the environment in main. It used to be one
// installation's bucket, written here.
var bucketName string

// firstEnv returns the first of the named variables that is set.
func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func main() {
	ctx := context.TODO()

	// The same settings file-grpc-service reads, so the script follows the
	// installation to whichever S3-compatible store it uses.
	bucketName = firstEnv("STORAGE_BUCKET", "AWS_FILES_S3_BUCKET")
	if bucketName == "" {
		log.Fatal("set STORAGE_BUCKET to the bucket whose files should be migrated")
	}

	var options []func(*config.LoadOptions) error
	if region := firstEnv("STORAGE_REGION", "AWS_REGION", "AWS_DEFAULT_REGION"); region != "" {
		options = append(options, config.WithRegion(region))
	}
	if key, secret := os.Getenv("STORAGE_ACCESS_KEY"), os.Getenv("STORAGE_SECRET_KEY"); key != "" && secret != "" {
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")))
	}

	cfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint := os.Getenv("STORAGE_ENDPOINT"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = os.Getenv("STORAGE_PATH_STYLE") == "true"
	})

	// Start the copying process
	if err := copyFiles(ctx, s3Client); err != nil {
		log.Fatalf("error copying files: %v", err)
	}
}

// Helper function to transform the old path into the new one based on the service type
func transformPath(oldKey string) (string, error) {
	// If the path ends with a trailing slash, it's likely a directory, so skip it
	if strings.HasSuffix(oldKey, "/") {
		return "", fmt.Errorf("skipping directory path: %s", oldKey)
	}

	var newKey string

	// Updated Regex patterns for matching paths based on different services
	avatarRegex := regexp.MustCompile(`assets/public/n(\d+)/(com_people|com_groups|com_socialengine)/avatars/(original|large|medium|small|square)(.*)`)
	coverRegex := regexp.MustCompile(`assets/public/n(\d+)/(com_people|com_groups|com_articles)(/com_articles)?/covers/(large|medium|original)(.*)`)
	photoRegex := regexp.MustCompile(`assets/public/n(\d+)/com_photos/(.*)_(original|large|xlarge|xxlarge|medium|small|square|thumbnail)\.(.*)`)

	if matches := avatarRegex.FindStringSubmatch(oldKey); len(matches) == 5 {
		// Avatar path (com_people, com_groups, or com_socialengine)
		nodeId := matches[1]
		service := "avatar-service"
		nodeType := "person"
		if matches[2] == "com_groups" {
			nodeType = "group"
		} else if matches[2] == "com_socialengine" {
			nodeType = "person" // Legacy path for person avatars
		}
		size := matches[3]
		filename := matches[4]

		// Extract the extension from the filename
		extension := getFileExtension(filename)

		// Format: <filename>_<size>.<extension>
		newKey = fmt.Sprintf("%s%s/%s/%s/%s_%s.%s", newPathPrefix, service, nodeType, nodeId, strings.TrimSuffix(filename, "."+extension), size, extension)

	} else if matches := coverRegex.FindStringSubmatch(oldKey); len(matches) == 6 {
		// Cover path (com_people, com_groups, or com_articles), handling the com_articles/com_articles issue
		nodeId := matches[1]
		service := "cover-service"
		nodeType := "person"
		if matches[2] == "com_groups" {
			nodeType = "group"
		} else if matches[2] == "com_articles" {
			nodeType = "article"
		}
		size := matches[4]
		filename := matches[5]

		// Extract the extension from the filename
		extension := getFileExtension(filename)

		// Format: <filename>_<size>.<extension>
		newKey = fmt.Sprintf("%s%s/%s/%s/%s_%s.%s", newPathPrefix, service, nodeType, nodeId, strings.TrimSuffix(filename, "."+extension), size, extension)

	} else if matches := photoRegex.FindStringSubmatch(oldKey); len(matches) == 5 {
		// Photo path (com_photos)
		nodeId := matches[1]
		service := "photo-service"
		nodeType := "photo"
		filename := matches[2] // Extract filename before the underscore and size
		size := matches[3]
		extension := matches[4] // Extract the file extension

		// Format: <filename>_<size>.<extension>
		newKey = fmt.Sprintf("%s%s/%s/%s/%s_%s.%s", newPathPrefix, service, nodeType, nodeId, filename, size, extension)

	} else {
		return "", fmt.Errorf("unrecognized file path format: %s", oldKey)
	}

	return newKey, nil
}

// Helper function to extract the file extension
func getFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func copyFiles(ctx context.Context, s3Client *s3.Client) error {
	var continuationToken *string

	for {
		// List objects in the bucket for all services (avatars, covers, photos)
		listInput := &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucketName),
			Prefix:            aws.String("assets/public/n"), // Filter based on your file path
			ContinuationToken: continuationToken,             // Used for pagination
		}
		listOutput, err := s3Client.ListObjectsV2(ctx, listInput)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		// Iterate over each object
		for _, object := range listOutput.Contents {
			oldKey := *object.Key
			newKey, err := transformPath(oldKey)
			if err != nil {
				log.Printf("Error transforming path for %s: %v\n", oldKey, err)
				continue
			}

			// Skip empty or directory paths
			if newKey == "" {
				log.Printf("Skipping directory or invalid path: %s\n", oldKey)
				continue
			}

			// Copy the object
			copyInput := &s3.CopyObjectInput{
				Bucket:     aws.String(bucketName),
				CopySource: aws.String(fmt.Sprintf("%s/%s", bucketName, oldKey)),
				Key:        aws.String(newKey),
			}
			_, err = s3Client.CopyObject(ctx, copyInput)
			if err != nil {
				log.Printf("Failed to copy object %s: %v\n", oldKey, err)
				continue
			}

			fmt.Printf("Copied %s to %s\n", oldKey, newKey)
		}

		// If there are no more objects to fetch, exit the loop
		if listOutput.IsTruncated != nil && !*listOutput.IsTruncated {
			break
		}

		// Update continuationToken for the next iteration
		continuationToken = listOutput.NextContinuationToken
	}

	return nil
}
