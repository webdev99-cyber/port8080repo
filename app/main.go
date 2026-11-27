package main

import (
	"bufio"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

const listenAddr = "127.0.0.1:8765"
const adminAddr = "127.0.0.1:8766"
const captureFPS = 30

//go:embed web/host.html
var hostHTML string

//go:embed web/viewer.html
var viewerHTML string

type App struct {
	token        string
	mu           sync.Mutex
	url          string
	tun          *exec.Cmd
	peer         *webrtc.PeerConnection
	viewerActive bool
}

type wireMessage struct {
	Type      string                   `json:"type"`
	SDP       string                   `json:"sdp,omitempty"`
	Message   string                   `json:"message,omitempty"`
	DeviceID  string                   `json:"deviceId,omitempty"`
	Candidate *webrtc.ICECandidateInit `json:"candidate,omitempty"`
}

func main() {
	token, err := randomHex(32)
	if err != nil {
		log.Fatal(err)
	}
	a := &App{token: token}
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/view/", a.viewerPage)
	publicMux.HandleFunc("/signal/", a.signal)
	publicServer := &http.Server{Addr: listenAddr, Handler: publicMux, ReadHeaderTimeout: 5 * time.Second}
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/host", a.hostPage)
	adminMux.HandleFunc("/host/status", a.hostStatus)
	adminMux.HandleFunc("/host/stop", a.hostStop)
	adminServer := &http.Server{Addr: adminAddr, Handler: adminMux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		if err := publicServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	go func() {
		if err := adminServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	if err := a.startTunnel(); err != nil {
		log.Fatal(err)
	}
	openLocal("http://127.0.0.1:8766/host")
	log.Printf("Port8080 host page: http://127.0.0.1:8766/host")
	select {}
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *App) startTunnel() error {
	pathURL := "http://" + listenAddr
	cmd := exec.Command(bundledTool("cloudflared"), "tunnel", "--url", pathURL, "--no-autoupdate")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start cloudflared: %w", err)
	}
	a.mu.Lock()
	a.tun = cmd
	a.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	lines := make(chan string, 16)
	read := func(r io.Reader) {
		s := bufio.NewScanner(r)
		for s.Scan() {
			lines <- s.Text()
		}
	}
	go read(stdout)
	go read(stderr)
	deadline := time.After(60 * time.Second)
	for {
		select {
		case line := <-lines:
			log.Printf("cloudflared: %s", line)
			for _, part := range strings.Fields(line) {
				if strings.HasPrefix(part, "https://") && strings.Contains(part, ".trycloudflare.com") {
					a.mu.Lock()
					a.url = strings.TrimRight(part, ".,)")
					a.mu.Unlock()
					go func() {
						err := <-done
						log.Printf("cloudflared stopped: %v", err)
						a.mu.Lock()
						if a.tun == cmd {
							a.url = ""
						}
						a.mu.Unlock()
					}()
					return nil
				}
			}
		case <-deadline:
			_ = cmd.Process.Kill()
			return errors.New("timed out waiting for Cloudflare Quick Tunnel URL; check cloudflared output")
		case err := <-done:
			return fmt.Errorf("cloudflared exited before publishing a URL: %w", err)
		}
	}
}

func openLocal(target string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		return
	}
	_ = cmd.Start()
}

func (a *App) hostPage(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r.RemoteAddr) || r.Host != adminAddr {
		http.Error(w, "local only", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, hostHTML)
}

func (a *App) hostStatus(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r.RemoteAddr) || r.Host != adminAddr {
		http.Error(w, "local only", http.StatusForbidden)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	viewerURL := ""
	if a.url != "" {
		viewerURL = a.url + "/view/" + a.token
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"url": viewerURL, "ready": a.url != ""})
}

func (a *App) hostStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !isLoopback(r.RemoteAddr) || r.Host != adminAddr {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	a.mu.Lock()
	if a.peer != nil {
		_ = a.peer.Close()
		a.peer = nil
	}
	if a.tun != nil && a.tun.Process != nil {
		_ = a.tun.Process.Kill()
		a.tun = nil
	}
	a.url = ""
	a.mu.Unlock()
	fmt.Fprint(w, `{"stopped":true}`)
	go func() { time.Sleep(250 * time.Millisecond); os.Exit(0) }()
}

func isLoopback(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	return err == nil && net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func (a *App) viewerPage(w http.ResponseWriter, r *http.Request) {
	if strings.TrimPrefix(r.URL.Path, "/view/") != a.token {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, viewerHTML)
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	u, err := url.Parse(r.Header.Get("Origin"))
	return err == nil && u.Host == r.Host
}}

func (a *App) signal(w http.ResponseWriter, r *http.Request) {
	if strings.TrimPrefix(r.URL.Path, "/signal/") != a.token {
		http.NotFound(w, r)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	a.mu.Lock()
	if a.viewerActive {
		a.mu.Unlock()
		_ = conn.WriteJSON(wireMessage{Type: "error", Message: "A viewer is already connected. Try again when that session ends."})
		return
	}
	a.viewerActive = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.viewerActive = false
		a.mu.Unlock()
		releaseAllInput()
	}()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}}})
	if err != nil {
		return
	}
	a.mu.Lock()
	a.peer = pc
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		if a.peer == pc {
			a.peer = nil
		}
		a.mu.Unlock()
		_ = pc.Close()
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			cancel()
		}
	})

	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e02a"}, "desktop", "desklink")
	if err != nil {
		return
	}
	if _, err = pc.AddTrack(track); err != nil {
		return
	}
	pc.OnDataChannel(func(dc *webrtc.DataChannel) { dc.OnMessage(func(m webrtc.DataChannelMessage) { handleInput(m.Data) }) })
	go a.capture(ctx, track)

	for {
		var msg wireMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		switch msg.Type {
		case "offer":
			if len(msg.DeviceID) <= 64 {
				log.Printf("viewer browser profile: %q", msg.DeviceID)
			}
			if err = pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: msg.SDP}); err != nil {
				return
			}
			answer, e := pc.CreateAnswer(nil)
			if e != nil {
				return
			}
			gather := webrtc.GatheringCompletePromise(pc)
			if e = pc.SetLocalDescription(answer); e != nil {
				return
			}
			<-gather
			if e = conn.WriteJSON(wireMessage{Type: "answer", SDP: pc.LocalDescription().SDP}); e != nil {
				return
			}
		case "candidate":
			if msg.Candidate != nil {
				_ = pc.AddICECandidate(*msg.Candidate)
			}
		}
	}
}

func (a *App) capture(ctx context.Context, track *webrtc.TrackLocalStaticSample) {
	candidates := []string{"h264_videotoolbox"}
	if runtime.GOOS == "windows" {
		candidates = []string{"h264_nvenc", "h264_amf", "h264_qsv", "h264_mf"}
	}
	for _, encoder := range candidates {
		if ctx.Err() != nil {
			return
		}
		if err := a.captureWithEncoder(ctx, track, encoder); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("capture encoder %s stopped: %v", encoder, err)
		}
	}
	log.Printf("no capture encoder produced a continuing stream")
}

func (a *App) captureWithEncoder(ctx context.Context, track *webrtc.TrackLocalStaticSample, encoder string) error {
	args := ffmpegArgs(encoder)
	cmd := exec.CommandContext(ctx, bundledTool("ffmpeg"), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	frames, err := accessUnits(stdout)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(time.Second / captureFPS)
	defer ticker.Stop()
	for frame := range frames {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		if len(frame) > 0 {
			_ = track.WriteSample(media.Sample{Data: frame, Duration: time.Second / captureFPS})
		}
	}
	waited = true
	return cmd.Wait()
}

func ffmpegArgs(encoder string) []string {
	profile := "baseline"
	if encoder == "h264_amf" {
		profile = "constrained_baseline"
	}
	common := []string{"-vf", "scale='min(1280,iw)':'min(720,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2,pad=1280:720:(ow-iw)/2:(oh-ih)/2", "-c:v", encoder, "-profile:v", profile, "-pix_fmt", "yuv420p", "-fps_mode", "passthrough", "-g", "30", "-bf", "0", "-b:v", "6M", "-color_range", "pc", "-colorspace", "bt709", "-color_primaries", "bt709", "-color_trc", "bt709", "-bsf:v", "h264_metadata=aud=insert", "-f", "h264", "pipe:1"}
	switch encoder {
	case "h264_videotoolbox":
		common = append([]string{"-realtime", "1", "-allow_sw", "0"}, common...)
	case "h264_nvenc":
		common = append([]string{"-preset", "p1", "-tune", "ull"}, common...)
	case "h264_amf":
		common = append([]string{"-usage", "ultralowlatency", "-quality", "speed"}, common...)
	case "h264_qsv":
		common = append([]string{"-preset", "veryfast"}, common...)
	case "h264_mf":
		common = append([]string{"-low_latency", "1"}, common...)
	case "libx264":
		common = append([]string{"-preset", "ultrafast", "-tune", "zerolatency"}, common...)
	}
	common = append([]string{"-hide_banner", "-loglevel", "warning", "-an"}, common...)
	if runtime.GOOS == "darwin" {
		display := os.Getenv("DESKLINK_MAC_DISPLAY")
		if display == "" {
			display = "1"
		}
		return append([]string{"-f", "avfoundation", "-capture_cursor", "1", "-framerate", fmt.Sprint(captureFPS), "-i", display + ":none"}, common...)
	}
	return append([]string{"-f", "gdigrab", "-framerate", fmt.Sprint(captureFPS), "-draw_mouse", "1", "-i", "desktop"}, common...)
}

// accessUnits turns FFmpeg's Annex-B stream into one sample per AUD-delimited frame.
func accessUnits(src io.Reader) (<-chan []byte, error) {
	out := make(chan []byte, 3)
	go func() {
		defer close(out)
		reader := bufio.NewReaderSize(src, 64*1024)
		buf := make([]byte, 0, 256*1024)
		chunk := make([]byte, 64*1024)
		for {
			n, err := reader.Read(chunk)
			if n > 0 {
				buf = append(buf, chunk[:n]...)
				buf = splitAUD(buf, out)
			}
			if err != nil {
				if len(buf) > 0 {
					out <- buf
				}
				return
			}
		}
	}()
	return out, nil
}

func splitAUD(b []byte, out chan<- []byte) []byte {
	starts := make([]int, 0, 8)
	for i := 0; i+4 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && ((b[i+2] == 1 && b[i+3]&0x1f == 9) || (i+5 < len(b) && b[i+2] == 0 && b[i+3] == 1 && b[i+4]&0x1f == 9)) {
			starts = append(starts, i)
		}
	}
	if len(starts) < 2 {
		return b
	}
	for j := 0; j < len(starts)-1; j++ {
		out <- append([]byte(nil), b[starts[j]:starts[j+1]]...)
	}
	return append([]byte(nil), b[starts[len(starts)-1]:]...)
}

func handleInput(data []byte) {
	inputMu.Lock()
	defer inputMu.Unlock()
	var m struct {
		Type   string  `json:"type"`
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Key    string  `json:"key"`
		Button string  `json:"button"`
		Delta  float64 `json:"delta"`
	}
	if json.Unmarshal(data, &m) != nil {
		return
	}
	w, h := robotgo.GetScreenSize()
	x, y := int(m.X*float64(w)), int(m.Y*float64(h))
	switch m.Type {
	case "move":
		robotgo.Move(x, y)
	case "down":
		if m.Button == "left" || m.Button == "middle" || m.Button == "right" {
			robotgo.Toggle(m.Button, "down", x, y)
			pressedButtons[m.Button] = struct{}{}
		}
	case "up":
		if m.Button == "left" || m.Button == "middle" || m.Button == "right" {
			robotgo.Toggle(m.Button, "up", x, y)
			delete(pressedButtons, m.Button)
		}
	case "keydown":
		if key := keyName(m.Key); key != "" {
			robotgo.KeyDown(key)
			pressedKeys[key] = struct{}{}
		}
	case "keyup":
		if key := keyName(m.Key); key != "" {
			robotgo.KeyUp(key)
			delete(pressedKeys, key)
		}
	case "scroll":
		robotgo.Scroll(0, int(m.Delta))
	}
}

func keyName(k string) string {
	if strings.HasPrefix(k, "Key") {
		return strings.ToLower(strings.TrimPrefix(k, "Key"))
	}
	if strings.HasPrefix(k, "Digit") {
		return strings.TrimPrefix(k, "Digit")
	}
	if strings.HasPrefix(k, "F") && len(k) <= 3 {
		return strings.ToLower(k)
	}
	switch k {
	case " ":
		return "space"
	case "Enter":
		return "enter"
	case "Escape":
		return "esc"
	case "Backspace":
		return "backspace"
	case "Tab":
		return "tab"
	case "Shift":
		return "shift"
	case "Control":
		return "ctrl"
	case "Alt":
		return "alt"
	case "Meta":
		return "cmd"
	case "ArrowUp":
		return "up"
	case "ArrowDown":
		return "down"
	case "ArrowLeft":
		return "left"
	case "ArrowRight":
		return "right"
	case "Delete":
		return "delete"
	case "Home":
		return "home"
	case "End":
		return "end"
	case "PageUp":
		return "pageup"
	case "PageDown":
		return "pagedown"
	case "Insert":
		return "insert"
	case "CapsLock":
		return "capslock"
	}
	if len(k) == 1 {
		return strings.ToLower(k)
	}
	return ""
}

var inputMu sync.Mutex
var pressedKeys = map[string]struct{}{}
var pressedButtons = map[string]struct{}{}

func releaseAllInput() {
	inputMu.Lock()
	defer inputMu.Unlock()
	for key := range pressedKeys {
		robotgo.KeyUp(key)
		delete(pressedKeys, key)
	}
	for button := range pressedButtons {
		robotgo.Toggle(button, "up")
		delete(pressedButtons, button)
	}
}

func bundledTool(name string) string {
	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), name)
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return name
}
