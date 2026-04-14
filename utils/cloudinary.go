package utils

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// Enabled returns true when all three Cloudinary env vars are present.
func CloudinaryEnabled() bool {
	return os.Getenv("CLOUDINARY_CLOUD_NAME") != "" &&
		os.Getenv("CLOUDINARY_API_KEY") != "" &&
		os.Getenv("CLOUDINARY_API_SECRET") != ""
}

func newCldClient() (*cloudinary.Cloudinary, error) {
	cld, err := cloudinary.NewFromParams(
		os.Getenv("CLOUDINARY_CLOUD_NAME"),
		os.Getenv("CLOUDINARY_API_KEY"),
		os.Getenv("CLOUDINARY_API_SECRET"),
	)
	if err != nil {
		return nil, fmt.Errorf("cloudinary init error: %w", err)
	}
	return cld, nil
}

// UploadImage uploads a remote image URL to Cloudinary and returns the permanent HTTPS URL.
// folder example: "furci/carousel"
// publicID example: "slide_1_day2" (empty string = auto-generated)
func UploadImage(ctx context.Context, sourceURL, folder, publicID string) (string, error) {
	cld, err := newCldClient()
	if err != nil {
		return "", err
	}

	params := uploader.UploadParams{
		Folder:       folder,
		ResourceType: "image",
	}
	if publicID != "" {
		params.PublicID = publicID
	}

	resp, err := cld.Upload.Upload(ctx, sourceURL, params)
	if err != nil {
		return "", fmt.Errorf("cloudinary image upload failed: %w", err)
	}

	return resp.SecureURL, nil
}

// UploadVideo uploads a remote video URL to Cloudinary and returns the permanent HTTPS URL.
// folder example: "furci/videos"
func UploadVideo(ctx context.Context, sourceURL, folder, publicID string) (string, error) {
	cld, err := newCldClient()
	if err != nil {
		return "", err
	}

	params := uploader.UploadParams{
		Folder:       folder,
		ResourceType: "video",
	}
	if publicID != "" {
		params.PublicID = publicID
	}

	resp, err := cld.Upload.Upload(ctx, sourceURL, params)
	if err != nil {
		return "", fmt.Errorf("cloudinary video upload failed: %w", err)
	}

	return resp.SecureURL, nil
}
