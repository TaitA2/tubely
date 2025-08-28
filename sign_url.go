package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	presignReq, err := presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{Bucket: &bucket, Key: &key}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", fmt.Errorf("Error getting object: %v", err)
	}

	return presignReq.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, fmt.Errorf("Error: video url is nil.")
	}
	// fmt.Println("VIDEO: ", video)
	elems := strings.Split(*video.VideoURL, ",")
	// fmt.Println("ELEMS: ", elems)
	bucket, key := elems[0], elems[1]
	fmt.Printf("BUCKET: %s, KEY: %s\n", bucket, key)
	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Hour)
	if err != nil {
		return video, fmt.Errorf("Error generating presigned url: %v", err)
	}
	video.VideoURL = &presignedURL
	return video, nil
}
