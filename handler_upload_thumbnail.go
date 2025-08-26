package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	defer file.Close()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	mediaType := header.Header.Get("Content-Type")
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusConflict, "Invalid filetype", err)
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

	// update

	b := make([]byte, 32)
	_, err = rand.Read(b)
	randURI := base64.RawURLEncoding.EncodeToString(b)
	fileURI := fmt.Sprintf("./assets/%v.%s", randURI, strings.Split(mediaType, "/")[1])
	newURL := fmt.Sprintf("http://localhost:%s/assets/%v.%s", cfg.port, randURI, strings.Split(mediaType, "/")[1])
	osFile, err := os.Create(fileURI)
	if err != nil {
		fmt.Printf("Error creating thumbnail file: %v\n", err)
		respondWithError(w, http.StatusInternalServerError, "Unable to create thumbnail file", err)
		return
	}
	_, err = io.Copy(osFile, file)
	if err != nil {
		fmt.Printf("Error saving thumbnail in os")
		respondWithError(w, http.StatusInternalServerError, "Unable to saving thumbnail in os", err)
		return
	}
	video.ThumbnailURL = &newURL
	err = cfg.db.UpdateVideo(video)
	fmt.Printf("Updated url: %v\n", *video.ThumbnailURL)
	if err != nil {
		fmt.Printf("Error updating video in db")
		respondWithError(w, http.StatusInternalServerError, "Unable to update video in db", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
