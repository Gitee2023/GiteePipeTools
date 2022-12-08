package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/klauspost/compress/zip"
	"github.com/sirupsen/logrus"
)

const envPrefix = "ARTIFACT_"

func main() {
	debug := flag.Bool("debug", false, "Debug mode")
	envPrefix := flag.String("envprefix", "ARTIFACT_", "Environment prefix")
	targetDir := flag.String("targetdir", "./", "Destination directory to extracted files")
	flag.Parse()

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	var artifactURLs []string
	for _, env := range os.Environ() {
		if strings.Contains(env, *envPrefix) {
			envSpl := strings.Split(env, "=")
			if len(envSpl) != 2 {
				logrus.Debugf("Environmet has no value: %s", envSpl[0])
				continue
			}
			artifactURLs = append(artifactURLs, envSpl[1])
		}
	}

	var actualTheads int
	maxThreads := (runtime.NumCPU() + 1) * 2
	errChan := make(chan error, len(artifactURLs))
	for _, url := range artifactURLs {
		for {
			if actualTheads == maxThreads {
				time.Sleep(time.Millisecond * 100)
			}
			break
		}

		go func(url string) {
			errChan <- downloadAndExtract(url, *targetDir)
		}(url)
		maxThreads++
	}

	for i := 0; i < cap(errChan); i++ {
		if err := <-errChan; err != nil {
			logrus.Errorf("Failed to extract: %v", err)
		}
	}
}

func downloadAndExtract(url string, targetDir string) error {
	fileName := urlToFileName(url)
	logrus.Debugf("Downloading file: %s", fileName)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get artifact: %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read artifact: %s: %w", fileName, err)
	}

	logrus.Debugf("Extracting file: %s", fileName)
	if err := unzipToFS(body, targetDir); err != nil {
		return fmt.Errorf("unzip artifact: %s: %w", fileName, err)
	}

	logrus.Infof("Successfully extracted file: %s", fileName)
	return nil
}

func unzipToFS(data []byte, targetDir string) error {
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	for _, file := range zipReader.File {
		zippedFile, err := file.Open()
		if err != nil {
			return err
		}
		defer zippedFile.Close()

		extractedFilePath := filepath.Join(
			targetDir,
			file.Name,
		)
		if file.FileInfo().IsDir() {
			os.MkdirAll(extractedFilePath, file.Mode())
		} else {
			outputFile, err := os.OpenFile(
				extractedFilePath,
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				file.Mode(),
			)
			if err != nil {
				return fmt.Errorf("open file: %s: %w", extractedFilePath, err)
			}
			defer outputFile.Close()

			_, err = io.Copy(outputFile, zippedFile)
			if err != nil {
				return fmt.Errorf("create file: %s: %w", extractedFilePath, err)
			}
		}
	}

	return nil
}

func urlToFileName(url string) string {
	dummyRequest, _ := http.NewRequest("GET", url, nil)
	return path.Base(dummyRequest.URL.Path)
}
