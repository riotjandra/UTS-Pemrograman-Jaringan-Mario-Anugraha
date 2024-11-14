package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Struktur User untuk menyimpan saldo setiap pengguna
type User struct {
	Wallet Wallet
}

// Struktur Wallet untuk saldo dan mutex
type Wallet struct {
	Balance int
	Mutex   sync.Mutex
}

// Menambahkan saldo ke dompet
func (w *Wallet) TopUp(amount int) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()
	w.Balance += amount
}

// Mendapatkan saldo saat ini
func (w *Wallet) GetBalance() int {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()
	return w.Balance
}

// Struktur Request untuk menerima permintaan
type Request struct {
	Type     string `json:"type"`     // "saldo", "topup", atau "donasi"
	Username string `json:"username"` // username pengguna
	Amount   int    `json:"amount"`   // nominal uang untuk topup atau donasi
	Message  string `json:"message"`  // pesan donasi
}

// Struktur pesan donasi
type DonationMessage struct {
	Message string `json:"message"`
	Amount  int    `json:"amount"`
}

// Variabel peta untuk menyimpan data pengguna
var users = map[string]*User{}
var usersMutex sync.Mutex // Mutex untuk melindungi akses ke peta pengguna

// Daftar klien WebSocket dan mutex untuk menghindari race condition
var clients = make([]*websocket.Conn, 0)
var clientsMutex sync.Mutex // Mutex untuk melindungi akses ke clients

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func getUser(username string) *User {
	usersMutex.Lock()
	defer usersMutex.Unlock()
	// Jika pengguna belum ada, buat entri baru
	if _, exists := users[username]; !exists {
		users[username] = &User{Wallet: Wallet{}}
	}
	return users[username]
}

func main() {
	go startUDPServer()
	go startWebSocketServer() // Memulai server WebSocket
	startTCPServer()
}

// Fungsi untuk server UDP (pengelolaan dompet)
func startUDPServer() {
	addr, err := net.ResolveUDPAddr("udp", ":8080")
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("Error listening on UDP:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Server UDP berjalan di port 8080...")

	for {
		buf := make([]byte, 1024)
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			continue
		}

		var request Request
		err = json.Unmarshal(buf[:n], &request)
		if err != nil {
			fmt.Println("Error decoding request:", err)
			continue
		}

		user := getUser(request.Username) // Mendapatkan data pengguna berdasarkan username

		var response string
		if request.Type == "saldo" {
			response = fmt.Sprintf("Saldo saat ini untuk %s: %d", request.Username, user.Wallet.GetBalance())
		} else if request.Type == "topup" {
			user.Wallet.TopUp(request.Amount)
			response = fmt.Sprintf("Top-up berhasil. Saldo saat ini untuk %s: %d", request.Username, user.Wallet.GetBalance())
		} else {
			response = "Permintaan tidak dikenal."
		}

		_, err = conn.WriteToUDP([]byte(response), clientAddr)
		if err != nil {
			fmt.Println("Error sending response:", err)
		}
	}
}

// Fungsi untuk memulai server TCP (pesan donasi)
func startTCPServer() {
	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		fmt.Println("Error starting TCP server:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Server TCP berjalan di port 9090...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting TCP connection:", err)
			continue
		}

		go handleTCPConnection(conn)
	}
}

// Menghandle koneksi TCP dan mem-broadcast pesan donasi ke WebSocket
func handleTCPConnection(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)

	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		fmt.Println("Error membaca data TCP:", err)
		return
	}

	var request Request
	err = json.Unmarshal(buf[:n], &request)
	if err != nil {
		fmt.Println("Error decoding request:", err)
		return
	}

	user := getUser(request.Username) // Mendapatkan data pengguna berdasarkan username

	if request.Type == "donasi" {
		user.Wallet.Mutex.Lock()
		if user.Wallet.Balance < request.Amount {
			user.Wallet.Mutex.Unlock()
			conn.Write([]byte("Saldo tidak mencukupi"))
			return
		}
		user.Wallet.Balance -= request.Amount
		user.Wallet.Mutex.Unlock()

		message := fmt.Sprintf("Pesan donasi dari %s: %s", request.Username, request.Message)
		broadcastMessage(message, request.Amount) // Memanggil broadcastMessage dari websocket.go

		conn.Write([]byte("Pesan donasi berhasil diterima"))
	}
}

// Fungsi untuk memulai server WebSocket
func startWebSocketServer() {
	// Menjalankan server WebSocket
	http.HandleFunc("/ws", handleWebSocket)
	fmt.Println("Server WebSocket berjalan di port 5500...")
	if err := http.ListenAndServe(":5500", nil); err != nil {
		fmt.Println("Gagal memulai server WebSocket:", err)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Gagal meng-upgrade koneksi:", err)
		return
	}
	defer conn.Close()

	// Tambahkan klien baru ke daftar klien
	addClient(conn)
	fmt.Println("Client WebSocket terhubung")

	// Hapus klien dari daftar saat koneksi ditutup
	defer removeClient(conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Client WebSocket terputus:", err)
			break
		}
	}
}

// Fungsi untuk menambahkan klien baru ke daftar klien dengan proteksi mutex
func addClient(conn *websocket.Conn) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	clients = append(clients, conn)
}

// Fungsi untuk menghapus klien dari daftar klien dengan proteksi mutex
func removeClient(conn *websocket.Conn) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for i := 0; i < len(clients); i++ {
		if clients[i] == conn {
			clients = append(clients[:i], clients[i+1:]...)
			break
		}
	}
}

// Fungsi untuk mem-broadcast pesan donasi ke semua klien WebSocket
func broadcastMessage(message string, amount int) {
	donation := DonationMessage{
		Message: message,
		Amount:  amount,
	}

	data, err := json.Marshal(donation)
	if err != nil {
		fmt.Println("Error encoding donation message:", err)
		return
	}

	// Broadcast ke setiap klien
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	for i := 0; i < len(clients); i++ {
		conn := clients[i]
		err := conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			fmt.Println("Error broadcasting message:", err)
			conn.Close()
			// Hapus klien yang terputus dari daftar
			clients = append(clients[:i], clients[i+1:]...)
			i-- // Kurangi indeks karena slice telah bergeser
		}
	}
}
