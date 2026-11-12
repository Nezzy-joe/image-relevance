package domain

import "errors"

var (
	ErrImageTooLarge = errors.New("image exceeds the configured upload limit")
	ErrInvalidImage  = errors.New("upload is not a supported, valid image")
	ErrNoTenant      = errors.New("development tenant is missing; run the seed command")
	ErrImageNotFound = errors.New("image not found")
)
