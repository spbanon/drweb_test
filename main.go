package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

const storeDir = "./store"

var mu sync.Mutex

func saveFile(fileHeader *multipart.FileHeader, preCallback func(), postCallback func(), clientHashes map[string]string) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasherMd5 := md5.New()
	hasherSha1 := sha1.New()
	hasherSha256 := sha256.New()
	hashers := []struct {
		name   string
		hasher io.Writer
	}{
		{"md5", hasherMd5},
		{"sha1", hasherSha1},
		{"sha256", hasherSha256},
	}

	if _, err := io.Copy(io.MultiWriter(hashers[0].hasher, hashers[1].hasher, hashers[2].hasher), file); err != nil {
		return "", err
	}

	calculatedHashes := map[string]string{
		"md5":    hex.EncodeToString(hasherMd5.Sum(nil)),
		"sha1":   hex.EncodeToString(hasherSha1.Sum(nil)),
		"sha256": hex.EncodeToString(hasherSha256.Sum(nil)),
	}

	for hashType, clientHash := range clientHashes {
		if calculatedHashes[hashType] != clientHash {
			return "", fmt.Errorf("%s hash mismatch", hashType)
		}
	}

	fileHash := calculatedHashes["sha256"]
	subDir := filepath.Join(storeDir, fileHash[:2])
	if err := os.MkdirAll(subDir, os.ModePerm); err != nil {
		return "", err
	}

	filePath := filepath.Join(subDir, fileHash)

	if preCallback != nil {
		preCallback()
	}

	mu.Lock()
	defer mu.Unlock()

	fileDest, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer fileDest.Close()

	file.Seek(0, io.SeekStart)
	if _, err := io.Copy(fileDest, file); err != nil {
		return "", err
	}

	if postCallback != nil {
		postCallback()
	}

	return fileHash, nil
}

func getFilePath(fileHash string) string {
	return filepath.Join(storeDir, fileHash[:2], fileHash)
}

func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

func verifyFileIntegrity(filePath, expectedHash string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}

	calculatedHash := hex.EncodeToString(hasher.Sum(nil))
	if calculatedHash != expectedHash {
		return errors.New("file integrity check failed")
	}
	return nil
}

func deleteFile(filePath string) error {
	mu.Lock()
	defer mu.Unlock()
	return os.Remove(filePath)
}

func main() {
	app := fiber.New()
	app.Use(logger.New())
	app.Use(cors.New())

	app.Post("/upload", func(c *fiber.Ctx) error {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file part in the request"})
		}

		clientHashes := map[string]string{
			"md5":    c.FormValue("md5"),
			"sha1":   c.FormValue("sha1"),
			"sha256": c.FormValue("sha256"),
		}

		for k, v := range clientHashes {
			if v == "" {
				delete(clientHashes, k)
			}
		}

		preCallback := func() {
			log.Println("Pre-processing file")
		}
		postCallback := func() {
			log.Println("Post-processing file")
		}

		fileHash, err := saveFile(fileHeader, preCallback, postCallback, clientHashes)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"hash": fileHash})
	})

	app.Get("/download/:fileHash", func(c *fiber.Ctx) error {
		fileHash := c.Params("fileHash")
		filePath := getFilePath(fileHash)

		if !fileExists(filePath) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "File not found"})
		}

		if err := verifyFileIntegrity(filePath, fileHash); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "File integrity check failed"})
		}

		return c.SendFile(filePath)
	})

	app.Delete("/delete/:fileHash", func(c *fiber.Ctx) error {
		fileHash := c.Params("fileHash")
		filePath := getFilePath(fileHash)

		if !fileExists(filePath) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "File not found"})
		}

		if err := deleteFile(filePath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"message": "File deleted"})
	})

	if err := os.MkdirAll(storeDir, os.ModePerm); err != nil {
		log.Fatalf("Could not create store directory: %v", err)
	}

	log.Fatal(app.Listen(":1337"))
}
