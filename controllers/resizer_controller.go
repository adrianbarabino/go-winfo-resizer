package controllers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	//"time"

	"github.com/chai2010/webp"
	"github.com/nfnt/resize"
)

// ResizeController maneja las solicitudes de redimensionamiento de imágenes
type ResizeController struct{}

// Directorio para almacenar las imágenes en caché
const cacheDir = "./cache"

// ResizeImage procesa la solicitud de redimensionamiento
func (rc *ResizeController) ResizeImage(w http.ResponseWriter, r *http.Request, filename, qualityStr, widthStr, heightStr, crop string) {
	log.Println("ResizeImage alcanzado")

	// Decodifica el nombre de archivo desde Base64 (si aplica)
	decodedImagePath, err := base64.URLEncoding.DecodeString(filename)
	var imagePath string
	if err != nil {
		log.Printf("Error al decodificar Base64: %v, usando valor original", err)
		imagePath = "ads/" + filename
	} else {
		imagePath = "uploads/" + string(decodedImagePath)
	}
	fullImagePath := "https://brotemedia.sfo3.cdn.digitaloceanspaces.com/winfo/" + imagePath

	// Convierte los valores de calidad, ancho y alto a números
	quality, err := strconv.Atoi(qualityStr)
	if err != nil || quality <= 0 {
		http.Error(w, "El valor de calidad no es válido", http.StatusBadRequest)
		return
	}
	width, err := strconv.Atoi(widthStr)
	if err != nil || width <= 0 {
		http.Error(w, "El valor de ancho no es válido", http.StatusBadRequest)
		return
	}
	height, err := strconv.Atoi(heightStr)
	if err != nil || height <= 0 {
		http.Error(w, "El valor de alto no es válido", http.StatusBadRequest)
		return
	}

	// Construye la clave de caché y ruta del archivo
	cacheFileName := fmt.Sprintf("%s_%dx%d_q%d_crop%s.webp", filename, width, height, quality, crop)
	cacheFilePath := filepath.Join(cacheDir, cacheFileName)

	// Intenta cargar la imagen desde el caché del disco
	if cachedData, err := os.ReadFile(cacheFilePath); err == nil {
		log.Println("Imagen encontrada en caché de disco")
		writeCachedImage(w, cachedData)
		return
	}

	// Descarga la imagen desde el CDN
	img, err := downloadAndDecodeImage(fullImagePath)
	if err != nil {
		log.Printf("Error al descargar o decodificar la imagen: %v", err)
		http.Error(w, "No se pudo procesar la imagen", http.StatusInternalServerError)
		return
	}

	// Redimensionar y recortar la imagen
	resizedImage := resizeAndCropImage(img, width, height, crop == "true")

	// Codifica la imagen a WebP
	webpData, err := encodeImage(resizedImage, quality, "webp")
	if err != nil {
		log.Printf("Error al codificar la imagen: %v", err)
		http.Error(w, "No se pudo codificar la imagen", http.StatusInternalServerError)
		return
	}

	// Almacena la imagen en el caché del disco
	if err := saveToDisk(cacheFilePath, webpData); err != nil {
		log.Printf("Error al guardar la imagen en el disco: %v", err)
	}

	// Responde con la imagen generada
	writeCachedImage(w, webpData)
}

// downloadAndDecodeImage descarga y decodifica una imagen desde la URL
func downloadAndDecodeImage(url string) (image.Image, error) {
	log.Printf("Descargando imagen desde: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error al descargar la imagen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("imagen no encontrada, código de estado: %d", resp.StatusCode)
	}

	// Decodifica según el formato
	contentType := resp.Header.Get("Content-Type")
	switch contentType {
	case "image/jpeg", "image/jpg":
		return jpeg.Decode(resp.Body)
	case "image/png":
		return png.Decode(resp.Body)
	case "image/webp":
		return webp.Decode(resp.Body)
	default:
		return nil, fmt.Errorf("formato de imagen no soportado: %s", contentType)
	}
}

// resizeAndCropImage redimensiona y recorta la imagen según sea necesario
func resizeAndCropImage(img image.Image, width, height int, crop bool) image.Image {
	if crop {
		originalRatio := float64(img.Bounds().Dx()) / float64(img.Bounds().Dy())
		requiredRatio := float64(width) / float64(height)

		var resized image.Image
		if originalRatio > requiredRatio {
			resized = resize.Resize(0, uint(height), img, resize.Lanczos3)
		} else {
			resized = resize.Resize(uint(width), 0, img, resize.Lanczos3)
		}

		startX := (resized.Bounds().Dx() - width) / 2
		startY := (resized.Bounds().Dy() - height) / 2
		return croppedImage(resized, startX, startY, width, height)
	}

	return resize.Resize(uint(width), 0, img, resize.Lanczos3)
}

// encodeImage convierte una imagen a un formato específico
func encodeImage(img image.Image, quality int, format string) ([]byte, error) {
	buf := new(bytes.Buffer)
	switch format {
	case "webp":
		err := webp.Encode(buf, img, &webp.Options{Quality: float32(quality)})
		return buf.Bytes(), err
	default:
		return nil, fmt.Errorf("formato no soportado: %s", format)
	}
}

// saveToDisk guarda datos binarios en un archivo
func saveToDisk(filePath string, data []byte) error {
	// Crea el directorio si no existe
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return fmt.Errorf("no se pudo crear el directorio: %w", err)
	}

	// Guarda los datos en el archivo
	return os.WriteFile(filePath, data, os.ModePerm)
}

// writeCachedImage escribe la imagen almacenada en caché en la respuesta HTTP
func writeCachedImage(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 año
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// croppedImage recorta una imagen en las dimensiones especificadas
func croppedImage(src image.Image, x, y, width, height int) image.Image {
	rect := image.Rect(x, y, x+width, y+height)
	cropped := image.NewRGBA(rect)
	draw.Draw(cropped, rect, src, image.Point{x, y}, draw.Src)
	return cropped
}
