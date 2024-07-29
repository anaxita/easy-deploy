package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

type Config struct {
	HTTPPort int `json:"http_port"`
}

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// RequestPayload представляет JSON-данные из запроса.
type RequestPayload struct {
	URL string `json:"url"`
}

// CloneAndBuild выполняет клонирование репозитория, сборку Docker-образа и запуск контейнера.
func CloneAndBuild(repoURL string) error {
	// Создание временного каталога для клонирования репозитория
	tempDir, err := os.MkdirTemp("", "repo-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Клонирование репозитория
	logger.Info("Cloning repository", "repoURL", repoURL, "temp_dir", tempDir)

	cloneCmd := exec.Command("git", "clone", repoURL, tempDir)
	if b, err := cloneCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone repository: %w: %s", err, string(b))
	}

	// Проверка наличия Dockerfile
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("Dockerfile not found in repository")
	}

	// Сборка Docker-образа
	logger.Info("Building Docker image")
	buildCmd := exec.Command("docker", "build", "-t", path.Base(repoURL), tempDir)
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}

	// Поиск свободного порта
	port, err := findFreePort()
	if err != nil {
		return fmt.Errorf("failed to find free port: %w", err)
	}
	logger.Info("Found free port", slog.Int("port", port))

	// Запуск Docker-контейнера
	logger.Info("Running Docker container")
	runCmd := exec.Command("docker", "run", "-d", "-p", fmt.Sprintf("%d:80", port), path.Base(repoURL))
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("failed to run Docker container: %w", err)
	}

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

	if err := CloneAndBuild(payload.URL); err != nil {
		logger.Error("Failed to process repository", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Repository successfully processed"))
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
		Handler:      mux,
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
