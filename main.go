package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Config struct {
	HTTPPort int `json:"http_port"`
}

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

// RequestPayload представляет JSON-данные из запроса.
type RequestPayload struct {
	URL string `json:"url"`
}

// CloneAndBuild выполняет клонирование репозитория, сборку Docker-образа и запуск контейнера.
func CloneAndBuild(repoURL *url.URL) error {
	// Создание временного каталога для клонирования репозитория
	tempDir, err := os.MkdirTemp("", "repo-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Клонирование репозитория
	logger.Info("cloning repository", "repo_url", repoURL.String(), "temp_dir", tempDir)

	cloneCmd := exec.Command("git", "clone", repoURL.String(), tempDir)
	if b, err := cloneCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone repository: %w: %s", err, string(b))
	}

	logger.Info("Repository cloned")

	// Проверка наличия Dockerfile
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found in repository")
	}

	logger.Info("dockerfile found", "path", dockerfilePath)

	// Получение хэша последнего коммита
	lastCommitHashCmd := exec.Command("git", "-C", tempDir, "rev-parse", "--short", "HEAD")
	b, err := lastCommitHashCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get last commit hash: %w: %s", err, string(b))
	}

	lastCommitShortHash := strings.TrimSpace(string(b))

	logger.Info("Last commit hash", "hash", lastCommitShortHash)

	// Сборка Docker-образа
	imageName := repoURL.Host + repoURL.Path
	imageTag := lastCommitShortHash
	imageFullName := fmt.Sprintf("%s:%s", imageName, imageTag)

	logger.Info("Building Docker image")
	buildCmd := exec.Command("docker", "build", "-t", imageFullName, tempDir)
	if b, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build Docker image: %w: %s", err, string(b))
	}

	logger.Info("Docker image built", "image", imageName)

	logger.Info("Checking if container is running")
	isContainerRunning := exec.Command("docker", "ps", "-q", "--filter", fmt.Sprintf("ancestor=%s", imageName))
	b, err = isContainerRunning.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check if container is running: %w: %s", err, string(b))
	}

	containerID := strings.TrimSpace(string(b))

	var port int
	if containerID != "" {
		logger.Info("Container is running", "containerID", containerID)

		portCmd := exec.Command("docker", "port", containerID)
		b, err := portCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to get container port: %w: %s", err, string(b))
		}

		b = bytes.TrimSpace(b)

		port, err = strconv.Atoi(strings.Split(string(b), ":")[1])
		if err != nil {
			return fmt.Errorf("failed to parse container port: %w", err)
		}

		// Удаление контейнера
		logger.Info("Removing container")
		rmCmd := exec.Command("docker", "rm", "-f", containerID)
		if b, err := rmCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to remove container: %w: %s", err, string(b))
		}
	} else {
		logger.Info("Container is not running, searching for free port")
		// Поиск свободного порта
		port, err = findFreePort()
		if err != nil {
			return fmt.Errorf("failed to find free port: %w", err)
		}

		logger.Info("Found free port", slog.Int("port", port))
	}

	// Запуск Docker-контейнера
	logger.Info("Running Docker container")
	runCmd := exec.Command("docker", "run", "-d", "-p", fmt.Sprintf("%d:80", port), imageFullName)
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("failed to run Docker container: %w", err)
	}

	logger.Info("Successfully ran Docker container", slog.Int("port", port))

	return nil
}

// findFreePort находит свободный порт, начиная с 3000.
func findFreePort() (int, error) {
	for port := 3000; port <= 65535; port++ {
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free ports found")
}

// handleDeploy обрабатывает HTTP-запросы.
func handleDeploy(w http.ResponseWriter, r *http.Request) {
	var payload RequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	logger.Info("Received request", "url", payload.URL)

	parsedURL, err := url.Parse(payload.URL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	if err := CloneAndBuild(parsedURL); err != nil {
		logger.Error("clone and build", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Repository successfully processed"))
}

func parseJsonConfig(configPath string) (config Config, err error) {
	config.HTTPPort = 80

	configFile, err := os.Open(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config, nil
		}

		return Config{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

func main() {
	c, err := parseJsonConfig("config.json")
	if err != nil {
		logger.Error("failed to parse config", "error", err)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /deploy", handleDeploy)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", c.HTTPPort),
		Handler:      CorsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Info("Starting server", "address", server.Addr)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server failed", "error", err)
		}
	}()

	osCh := make(chan os.Signal, 1)
	signal.Notify(osCh, syscall.SIGTERM, syscall.SIGINT)
	<-osCh

	logger.Info("Shutting down server")

	if err := server.Shutdown(context.Background()); err != nil {
		logger.Error("Failed to shutdown server", "error", err)
	}

	logger.Info("Server stopped")
}

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
