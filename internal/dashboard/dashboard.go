package dashboard

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/eltonsst/tren/internal/stats"
)

//go:embed web/index.html web/styles.css
var webFiles embed.FS

type Handler struct {
	dataset      stats.Dataset
	loader       func(context.Context) (stats.Dataset, error)
	initialQuery string
	template     *template.Template
	styles       []byte
}

func New(dataset stats.Dataset, initialQuery string) (*Handler, error) {
	initialID := ""
	if strings.TrimSpace(initialQuery) != "" {
		exercise, err := stats.FindExercise(dataset, initialQuery)
		if err != nil {
			return nil, err
		}
		initialID = exercise.ID
	}
	pageTemplate, err := template.ParseFS(webFiles, "web/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse dashboard template: %w", err)
	}
	styles, err := webFiles.ReadFile("web/styles.css")
	if err != nil {
		return nil, fmt.Errorf("read dashboard styles: %w", err)
	}
	return &Handler{dataset: dataset, initialQuery: initialID, template: pageTemplate, styles: styles}, nil
}

func NewDynamic(loader func(context.Context) (stats.Dataset, error), initialQuery string) (*Handler, error) {
	if loader == nil {
		return nil, fmt.Errorf("dashboard dataset loader is required")
	}
	pageTemplate, err := template.ParseFS(webFiles, "web/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse dashboard template: %w", err)
	}
	styles, err := webFiles.ReadFile("web/styles.css")
	if err != nil {
		return nil, fmt.Errorf("read dashboard styles: %w", err)
	}
	return &Handler{
		loader: loader, initialQuery: strings.TrimSpace(initialQuery),
		template: pageTemplate, styles: styles,
	}, nil
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request.URL.Path == "/assets/styles.css" {
		writer.Header().Set("Content-Type", "text/css; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		_, _ = writer.Write(h.styles)
		return
	}
	dataset := h.dataset
	if h.loader != nil {
		var err error
		dataset, err = h.loader(request.Context())
		if err != nil {
			http.Error(writer, "load dashboard data", http.StatusServiceUnavailable)
			return
		}
	}
	selectedID, ok := h.selectedExercise(dataset, request.URL.Path)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	page, err := buildPage(dataset, selectedID, request.URL.Query().Get("q"))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := h.template.ExecuteTemplate(writer, "index.html", page); err != nil {
		http.Error(writer, "render dashboard", http.StatusInternalServerError)
	}
}

func (h *Handler) selectedExercise(dataset stats.Dataset, path string) (string, bool) {
	if path == "/" {
		if h.initialQuery == "" {
			return "", true
		}
		exercise, err := stats.FindExercise(dataset, h.initialQuery)
		if err != nil {
			return "", true
		}
		return exercise.ID, true
	}
	const prefix = "/exercise/"
	if !strings.HasPrefix(path, prefix) || strings.Contains(strings.TrimPrefix(path, prefix), "/") {
		return "", false
	}
	id := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	return id, id != ""
}

type ServeOptions struct {
	Address     string
	OpenBrowser bool
}

func Serve(ctx context.Context, dataset stats.Dataset, initialQuery string, options ServeOptions) error {
	handler, err := New(dataset, initialQuery)
	if err != nil {
		return err
	}
	address := strings.TrimSpace(options.Address)
	if address == "" {
		address = "127.0.0.1:8787"
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("start dashboard on %s: %w", address, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	url := "http://" + listener.Addr().String()
	if initialQuery != "" {
		if exercise, findErr := stats.FindExercise(dataset, initialQuery); findErr == nil {
			url += "/exercise/" + exercise.ID
		}
	}
	fmt.Printf("Tren dashboard: %s\nPress Ctrl-C to stop.\n", url)
	if options.OpenBrowser {
		_ = openURL(url)
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return err
}

func openURL(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	return command.Start()
}
