package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxBytes = 1 << 30

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unable to retrieve video metadata from db", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unable to retrieve video metadata from db", err)
		return
	}

	file, header, err := r.FormFile("video")
	defer file.Close()
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		fmt.Println("Error parsing media type: ", err)
		respondWithError(w, http.StatusInternalServerError, "Error parsing media type", err)
		return
	}
	fmt.Println("Media Type: ", mediaType)
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusConflict, "Invalid filetype", err)
		return
	}

	// temp file
	osFile, err := os.CreateTemp("", "tubely-upload.mp4")
	fmt.Println("OS File: ", osFile.Name())
	if err != nil {
		log.Fatalf("Error creating temp file: %v", err)
	}
	defer os.Remove(osFile.Name())
	defer osFile.Close()
	_, err = io.Copy(osFile, file)
	if err != nil {
		fmt.Printf("Error saving video in os")
		respondWithError(w, http.StatusInternalServerError, "Unable to save video in os", err)
		return
	}

	// fileKey
	b := make([]byte, 32)
	_, err = rand.Read(b)
	fileKey := base64.RawURLEncoding.EncodeToString(b) + ".mp4"
	fmt.Println("File Key: ", fileKey)

	osFile.Seek(0, io.SeekStart)
	_, err = cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        osFile,
		ContentType: &mediaType,
	})
	if err != nil {
		fmt.Printf("Error saving video in bucket")
		respondWithError(w, http.StatusInternalServerError, "Unable to save video in bucket", err)
		return
	}
	newURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileKey)
	video.VideoURL = &newURL
	err = cfg.db.UpdateVideo(video)

}
