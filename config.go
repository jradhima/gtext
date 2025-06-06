package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const CONFIGFILE string = ".gtext.conf"

type Config struct {
	ShowLineNumbers bool
	ExpandTabs      bool
	TabSize         int
}

func LoadConfig() Config {
	cfg := Config{
		ShowLineNumbers: true,
		ExpandTabs:      false,
		TabSize:         4,
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	file, err := os.Open(filepath.Join(home, CONFIGFILE))

	if err != nil {
		return cfg
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "show_line_numbers":
			cfg.ShowLineNumbers = val == "true"
		case "expand_tabs":
			cfg.ExpandTabs = val == "true"
		case "tab_size":
			if ts, err := strconv.Atoi(val); err == nil {
				cfg.TabSize = ts
			}
		default:
			fmt.Printf("Warning: unknown config key '%s'\n", key)
		}
	}

	return cfg
}

func promptUser(prompt string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [%s]: ", prompt, defaultValue)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input:", err)
		return defaultValue
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

func InitConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error: could not determine home directory:", err)
		os.Exit(1)
	}

	configPath := filepath.Join(home, CONFIGFILE)

	showLineNumbers := promptUser("Show line numbers (true/false)", "true")
	expandTabs := promptUser("Expand tabs to spaces (true/false)", "false")
	tabSize := promptUser("Tab size (number)", "4")
	tabSizeInt, err := strconv.Atoi(tabSize)
	if err != nil || tabSizeInt <= 0 {
		fmt.Println("Invalid tab size; using default (4)")
		tabSize = "4"
	}

	configContent := fmt.Sprintf(`# gtext config file
show_line_numbers=%s
expand_tabs=%s
tab_size=%s
`, showLineNumbers, expandTabs, tabSize)

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		fmt.Println("Error writing config:", err)
		os.Exit(1)
	}

	fmt.Printf("Created config at %s\n", configPath)
}
