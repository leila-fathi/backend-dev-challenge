package upload

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"net/http"

	_ "image/gif"

	"golang.org/x/image/draw"
)

func DetectMimeType(fileBytes []byte) string {
	return http.DetectContentType(fileBytes)
}

func CreateThumbnail(fileBytes []byte, maxSize int) ([]byte, error) {
	srcImage, _, err := image.Decode(bytes.NewReader(fileBytes))
	if err != nil {
		return nil, errors.New("unsupported image format")
	}

	bounds := srcImage.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	if srcWidth <= 0 || srcHeight <= 0 {
		return nil, errors.New("invalid image dimensions")
	}

	targetWidth, targetHeight := fitWithin(srcWidth, srcHeight, maxSize)
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), srcImage, bounds, draw.Over, nil)

	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func fitWithin(width, height, maxSize int) (int, int) {
	if width <= maxSize && height <= maxSize {
		return width, height
	}
	if width >= height {
		targetWidth := maxSize
		targetHeight := int(float64(height) * (float64(maxSize) / float64(width)))
		if targetHeight < 1 {
			targetHeight = 1
		}
		return targetWidth, targetHeight
	}
	targetHeight := maxSize
	targetWidth := int(float64(width) * (float64(maxSize) / float64(height)))
	if targetWidth < 1 {
		targetWidth = 1
	}
	return targetWidth, targetHeight
}
