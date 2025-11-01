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
	ScrollMargin    int
}

func DefaultConfig() *Config {
	cfg := Config{
		ShowLineNumbers: true,
		ExpandTabs:      false,
		TabSize:         4,
		ScrollMargin:    5,
	}
	return &cfg
}

func loadConfig() *Config {
	cfg := DefaultConfig()

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
			if b, err := strconv.ParseBool(val); err == nil {
				cfg.ShowLineNumbers = b
			}
		case "expand_tabs":
			if b, err := strconv.ParseBool(val); err == nil {
				cfg.ExpandTabs = b
			}
		case "tab_size":
			if ts, err := strconv.Atoi(val); err == nil && ts > 0 {
				cfg.TabSize = ts
			}
		case "scroll_margin":
			if sm, err := strconv.Atoi(val); err == nil && sm >= 0 {
				cfg.ScrollMargin = sm
			}
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

func initConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error: could not determine home directory:", err)
		os.Exit(1)
	}
	configPath := filepath.Join(home, CONFIGFILE)

	defaults := DefaultConfig()

	var showLineNumbersBool bool
	for {
		prompt := "Show line numbers (true/false)"
		input := promptUser(prompt, fmt.Sprintf("%t", defaults.ShowLineNumbers))
		if b, err := strconv.ParseBool(input); err == nil {
			showLineNumbersBool = b
			break // Valid boolean, exit loop
		}
		fmt.Println("Invalid input. Please enter 'true' or 'false'.")
	}

	var expandTabsBool bool
	for {
		prompt := "Expand tabs to spaces (true/false)"
		input := promptUser(prompt, fmt.Sprintf("%t", defaults.ExpandTabs))
		if b, err := strconv.ParseBool(input); err == nil {
			expandTabsBool = b
			break
		}
		fmt.Println("Invalid input. Please enter 'true' or 'false'.")
	}

	var tabSizeInt int
	for {
		prompt := "Tab size (number > 0)"
		input := promptUser(prompt, fmt.Sprintf("%d", defaults.TabSize))
		if ts, err := strconv.Atoi(input); err == nil && ts > 0 {
			tabSizeInt = ts
			break
		}
		fmt.Println("Invalid input. Please enter a number greater than 0.")
	}

	var scrollMarginInt int
	for {
		prompt := "Scroll margin (number >= 0)"
		input := promptUser(prompt, fmt.Sprintf("%d", defaults.ScrollMargin))
		if sm, err := strconv.Atoi(input); err == nil && sm >= 0 {
			scrollMarginInt = sm
			break
		}
		fmt.Println("Invalid input. Please enter a number 0 or greater.")
	}

	configContent := fmt.Sprintf(
		`# gtext config file
show_line_numbers=%t
expand_tabs=%t
tab_size=%d
scroll_margin=%d
`, showLineNumbersBool, expandTabsBool, tabSizeInt, scrollMarginInt)

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		fmt.Println("Error writing config:", err)
		os.Exit(1)
	}

	fmt.Printf("Created config at %s\n", configPath)
}
