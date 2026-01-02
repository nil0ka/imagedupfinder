package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"imagedupfinder/internal/server"
)

var (
	servePort    int
	serveTimeout time.Duration
	serveNoBrowser bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web UI for comparing and cleaning duplicates",
	Long: `Start a local web server that provides a visual interface for
comparing duplicate images and cleaning them.

The server will:
- Display duplicate groups with image previews
- Allow selecting which images to keep
- Execute clean operations from the browser
- Auto-shutdown after idle timeout (when tab is inactive)

Example:
  imagedupfinder serve              # Start on default port 8080
  imagedupfinder serve -p 3000      # Use custom port
  imagedupfinder serve --timeout 10m  # 10 minute idle timeout`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "Port to listen on")
	serveCmd.Flags().DurationVar(&serveTimeout, "timeout", 5*time.Minute, "Idle timeout (0 to disable)")
	serveCmd.Flags().BoolVar(&serveNoBrowser, "no-browser", false, "Don't open browser automatically")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	srv, err := server.New(dbPath, servePort, serveTimeout)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	url := fmt.Sprintf("http://localhost:%d", servePort)
	fmt.Printf("Starting server at %s\n", url)
	fmt.Printf("Idle timeout: %v (resets on activity, pauses when tab is active)\n", serveTimeout)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Open browser
	if !serveNoBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser(url)
		}()
	}

	return srv.Start()
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Run()
}
