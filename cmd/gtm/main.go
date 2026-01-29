package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultSessionPrefix = "gtm-"
	defaultWindowName    = "claude"
	restartEnvVar        = "GTM_RESTART_CMD"
)

type hookSettings struct {
	Hooks map[string][]hookMatcher `json:"hooks"`
}

type hookMatcher struct {
	Matcher string     `json:"matcher"`
	Hooks   []hookItem `json:"hooks"`
}

type hookItem struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "start":
		runStart(os.Args[2:])
	case "hook-stop":
		runHookStop(os.Args[2:])
	case "stop":
		runStop(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println(`gtm - gastown-micro

Usage:
  gtm init [--force]
  gtm start [--session <name>] [--cmd <command>]
  gtm hook-stop
  gtm stop [--session <name>]

Commands:
  init       Install minimal Claude hooks in ./.claude/settings.json
  start      Launch Claude Code inside a tmux session/window
  hook-stop  Restart Claude in the same tmux pane (used by Stop hook)
  stop       Kill the tmux session
`)
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "overwrite existing settings.json")
	_ = fs.Parse(args)

	root, err := os.Getwd()
	if err != nil {
		fatalf("get cwd: %v", err)
	}

	settingsPath := filepath.Join(root, ".claude", "settings.json")
	settings := hookSettings{
		Hooks: map[string][]hookMatcher{
			"SessionStart": {
				{
					Matcher: "",
					Hooks: []hookItem{{
						Type:    "command",
						Command: "bd prime",
					}},
				},
			},
			"Stop": {
				{
					Matcher: "",
					Hooks: []hookItem{{
						Type:    "command",
						Command: "gtm hook-stop",
					}},
				},
			},
		},
	}

	if !*force {
		if data, err := os.ReadFile(settingsPath); err == nil {
			var existing hookSettings
			if err := json.Unmarshal(data, &existing); err == nil {
				settings = mergeHookSettings(existing, settings)
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		fatalf("create .claude dir: %v", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fatalf("marshal settings: %v", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		fatalf("write %s: %v", settingsPath, err)
	}

	fmt.Printf("Wrote %s\n", settingsPath)
}

func mergeHookSettings(existing, added hookSettings) hookSettings {
	result := hookSettings{Hooks: map[string][]hookMatcher{}}

	for k, v := range existing.Hooks {
		result.Hooks[k] = append([]hookMatcher{}, v...)
	}
	for k, v := range added.Hooks {
		result.Hooks[k] = append(result.Hooks[k], v...)
	}

	return result
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	session := fs.String("session", "", "tmux session name (default: gtm-<repo>)")
	cmd := fs.String("cmd", "", "command to run in tmux (default: claude --dangerously-skip-permissions)")
	_ = fs.Parse(args)

	root, err := os.Getwd()
	if err != nil {
		fatalf("get cwd: %v", err)
	}

	sessName := *session
	if sessName == "" {
		sessName = defaultSessionPrefix + filepath.Base(root)
	}

	claudeCmd := strings.TrimSpace(*cmd)
	if claudeCmd == "" {
		claudeCmd = "claude --dangerously-skip-permissions"
	}

	if err := ensureTmux(); err != nil {
		fatalf("tmux not available: %v", err)
	}

	exists, err := tmuxHasSession(sessName)
	if err != nil {
		fatalf("checking tmux session: %v", err)
	}

	if !exists {
		if err := tmuxNewSession(sessName, defaultWindowName, root); err != nil {
			fatalf("create tmux session: %v", err)
		}
	} else {
		if err := tmuxEnsureWindow(sessName, defaultWindowName, root); err != nil {
			fatalf("ensure tmux window: %v", err)
		}
	}

	if err := tmuxSetEnv(sessName, restartEnvVar, claudeCmd); err != nil {
		fatalf("set tmux env: %v", err)
	}

	if err := tmuxRespawnWindow(sessName, defaultWindowName, claudeCmd); err != nil {
		fatalf("start claude: %v", err)
	}

	if err := tmuxAttachOrSwitch(sessName); err != nil {
		fatalf("attach tmux: %v", err)
	}
}

func runHookStop(args []string) {
	fs := flag.NewFlagSet("hook-stop", flag.ExitOnError)
	_ = fs.Parse(args)

	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		fatalf("TMUX_PANE not set; hook-stop must run inside tmux")
	}

	cmd, err := tmuxGetEnv(pane, restartEnvVar)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		fatalf("%s not set in tmux environment", restartEnvVar)
	}

	if err := tmuxRespawnPane(pane, cmd); err != nil {
		fatalf("respawn pane: %v", err)
	}
}

func runStop(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	session := fs.String("session", "", "tmux session name (default: gtm-<repo>)")
	_ = fs.Parse(args)

	root, err := os.Getwd()
	if err != nil {
		fatalf("get cwd: %v", err)
	}

	sessName := *session
	if sessName == "" {
		sessName = defaultSessionPrefix + filepath.Base(root)
	}

	if err := tmuxKillSession(sessName); err != nil {
		fatalf("kill session: %v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func ensureTmux() error {
	_, err := exec.LookPath("tmux")
	return err
}

func tmuxHasSession(name string) (bool, error) {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func tmuxNewSession(name, window, workDir string) error {
	args := []string{"new-session", "-d", "-s", name, "-n", window}
	if workDir != "" {
		args = append(args, "-c", workDir)
	}
	return exec.Command("tmux", args...).Run()
}

func tmuxEnsureWindow(session, window, workDir string) error {
	cmd := exec.Command("tmux", "list-windows", "-t", session, "-F", "#{window_name}")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == window {
			return nil
		}
	}

	args := []string{"new-window", "-t", session, "-n", window}
	if workDir != "" {
		args = append(args, "-c", workDir)
	}
	return exec.Command("tmux", args...).Run()
}

func tmuxSetEnv(target, key, value string) error {
	return exec.Command("tmux", "set-environment", "-t", target, key, value).Run()
}

func tmuxGetEnv(target, key string) (string, error) {
	cmd := exec.Command("tmux", "show-environment", "-t", target, key)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", nil
	}
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", nil
	}
	return parts[1], nil
}

func tmuxRespawnWindow(session, window, command string) error {
	return exec.Command("tmux", "respawn-pane", "-k", "-t", fmt.Sprintf("%s:%s", session, window), "sh", "-lc", command).Run()
}

func tmuxRespawnPane(pane, command string) error {
	return exec.Command("tmux", "respawn-pane", "-k", "-t", pane, "sh", "-lc", command).Run()
}

func tmuxAttachOrSwitch(session string) error {
	if os.Getenv("TMUX") != "" {
		return exec.Command("tmux", "switch-client", "-t", session).Run()
	}
	return exec.Command("tmux", "attach-session", "-t", session).Run()
}

func tmuxKillSession(session string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", session)
	return cmd.Run()
}
