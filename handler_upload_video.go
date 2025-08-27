package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"

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
	ratio, err := getVideoAspectRatio(osFile.Name())
	if err != nil {
		fmt.Printf("Error getting aspect ratio")
		respondWithError(w, http.StatusInternalServerError, "Unable to get aspect ratio", err)
		return
	}
	fileKey = ratio + fileKey
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

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var byteBuffer bytes.Buffer
	cmd.Stdout = &byteBuffer
	cmd.Run()
	var metadata ffprobe
	if err := json.Unmarshal(byteBuffer.Bytes(), &metadata); err != nil {
		return "", fmt.Errorf("Error unmarshalling ffprobe output: %v", err)
	}
	width := metadata.Streams[0].Width
	height := metadata.Streams[0].Height
	ratio := getRatio(width, height)
	return ratio, nil
}

func getRatio(width, height int) string {
	if math.Round(16/(float64(width)/float64(height))) == 9 {
		return "landscape"
	}
	if math.Round(9/(float64(width)/float64(height))) == 16 {
		return "portrait"
	}
	return "other"
}

type ffprobe struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
}
