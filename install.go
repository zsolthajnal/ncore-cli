package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const unitTemplate = `[Unit]
Description=ncore series watcher
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/ncore-cli/env
ExecStart={{.ExecPath}} watch \
  --media-dir {{.MediaDir}} \
  --torrent-dir {{.TorrentDir}} \
  --state {{.StateFile}} \
  --interval {{.Interval}} \
  --download-cmd "{{.DownloadCmd}}"
Restart=on-failure
RestartSec=60
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`

func runInstall(cfg WatchConfig) error {
	// Must be root
	if os.Geteuid() != 0 {
		return fmt.Errorf("install must be run as root (sudo ncore-cli install ...)")
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	execPath, _ = filepath.Abs(execPath)

	// Create /etc/ncore-cli/
	if err := os.MkdirAll("/etc/ncore-cli", 0700); err != nil {
		return fmt.Errorf("create /etc/ncore-cli: %w", err)
	}

	// Write env file (credentials) — only if it doesn't already exist
	envPath := "/etc/ncore-cli/env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		content := fmt.Sprintf("NCORE_USER=%s\nNCORE_PASS=%s\n", cfg.NcoreUser, cfg.NcorePass)
		if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("write env file: %w", err)
		}
		if cfg.NcoreUser == "" || cfg.NcorePass == "" {
			fmt.Printf("Credentials template written to %s — fill in before starting the service.\n", envPath)
		} else {
			fmt.Printf("Credentials written to %s\n", envPath)
		}
	} else {
		fmt.Printf("Env file %s already exists, not overwriting\n", envPath)
	}

	// Create state dir
	if err := os.MkdirAll(filepath.Dir(cfg.StateFile), 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	// Generate the systemd unit file
	tmpl, err := template.New("unit").Parse(unitTemplate)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		ExecPath    string
		MediaDir    string
		TorrentDir  string
		StateFile   string
		Interval    string
		DownloadCmd string
	}{
		ExecPath:    execPath,
		MediaDir:    cfg.MediaDir,
		TorrentDir:  cfg.TorrentDir,
		StateFile:   cfg.StateFile,
		Interval:    cfg.Interval.String(),
		DownloadCmd: cfg.DownloadCmd,
	}); err != nil {
		return err
	}

	unitPath := "/etc/systemd/system/ncore-cli.service"
	if err := os.WriteFile(unitPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	fmt.Printf("Unit file written to %s\n", unitPath)

	credsMissing := cfg.NcoreUser == "" || cfg.NcorePass == ""

	// Reload systemd and enable the service; only start if credentials are present.
	steps := [][]string{{"daemon-reload"}, {"enable", "ncore-cli"}}
	if !credsMissing {
		steps = append(steps, []string{"start", "ncore-cli"})
	}
	for _, args := range steps {
		cmd := exec.Command("systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("systemctl %v: %w", args, err)
		}
	}

	fmt.Println()
	if credsMissing {
		fmt.Println("ncore-cli service installed but NOT started — credentials are missing.")
		fmt.Println()
		fmt.Printf("  Edit : %s\n", envPath)
		fmt.Println("         Set NCORE_USER and NCORE_PASS, then run:")
		fmt.Println("  Start: systemctl start ncore-cli")
	} else {
		fmt.Println("ncore-cli service installed and started.")
		fmt.Println("  Status : systemctl status ncore-cli")
		fmt.Println("  Logs   : journalctl -fu ncore-cli")
	}
	fmt.Println("  Uninstall: sudo ncore-cli uninstall")
	return nil
}

func runUninstall() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("uninstall must be run as root (sudo ncore-cli uninstall)")
	}

	// Stop and disable the service (ignore errors if already stopped/missing)
	for _, args := range [][]string{
		{"stop", "ncore-cli"},
		{"disable", "ncore-cli"},
	} {
		cmd := exec.Command("systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	// Remove the unit file
	unitPath := "/etc/systemd/system/ncore-cli.service"
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}
	fmt.Printf("Removed %s\n", unitPath)

	// Reload systemd
	cmd := exec.Command("systemctl", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}

	fmt.Println("\nncore-cli service removed.")
	fmt.Println("  Credentials kept at : /etc/ncore-cli/env")
	fmt.Println("  State kept at       : /var/lib/ncore-cli/state.json")
	fmt.Println("  Remove manually if no longer needed.")
	return nil
}
