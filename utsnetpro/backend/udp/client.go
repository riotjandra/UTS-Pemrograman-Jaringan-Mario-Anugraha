package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Request struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Password string `json:"password"`
	Amount   int    `json:"amount"`
	Message  string `json:"message"`
}

var username, password string

func main() {
	reader := bufio.NewReader(os.Stdin)
	login(reader)

	for {
		fmt.Println("Pilih opsi:")
		fmt.Println("1. Lihat Saldo")
		fmt.Println("2. Top-Up Saldo")
		fmt.Println("3. Kirim Donasi")
		fmt.Println("4. Keluar") // Tambahkan opsi keluar
		fmt.Print("Masukkan pilihan (1-4): ")

		option, _ := reader.ReadString('\n')
		option = strings.TrimSpace(option)

		switch option {
		case "1":
			checkBalance()
		case "2":
			fmt.Print("Masukkan nominal top-up: ")
			amount := getAmount(reader)
			topUpBalance(amount)
		case "3":
			fmt.Print("Masukkan pesan donasi: ")
			message, _ := reader.ReadString('\n')
			message = strings.TrimSpace(message)
			fmt.Print("Masukkan nominal donasi: ")
			amount := getAmount(reader)
			sendDonation(message, amount)
		case "4":
			fmt.Println("Keluar dari program Donasy.")
			os.Exit(0) // Keluar dari program
		default:
			fmt.Println("Pilihan tidak valid.")
		}
	}
}

func login(reader *bufio.Reader) {
	fmt.Println("===== Program Donasy =====")

	fmt.Print("Masukkan username: ")
	username, _ = reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Masukkan password: ")
	password, _ = reader.ReadString('\n')
	password = strings.TrimSpace(password)
}

func checkBalance() {
	request := Request{Type: "saldo", Username: username, Password: password}
	sendUDPRequest(request)
}

func topUpBalance(amount int) {
	request := Request{Type: "topup", Username: username, Password: password, Amount: amount}
	sendUDPRequest(request)
}

func sendDonation(message string, amount int) {
	request := Request{Type: "donasi", Username: username, Password: password, Amount: amount, Message: message}
	sendTCPRequest(request)
}

func sendUDPRequest(request Request) {
	conn, err := net.Dial("udp", "localhost:8080")
	if err != nil {
		fmt.Println("Error connecting to UDP server:", err)
		return
	}
	defer conn.Close()

	data, err := json.Marshal(request)
	if err != nil {
		fmt.Println("Error encoding request:", err)
		return
	}

	_, err = conn.Write(data)
	if err != nil {
		fmt.Println("Error sending data:", err)
		return
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading response from server:", err)
		return
	}

	fmt.Println("Response dari server:", string(buf[:n]))
}

func sendTCPRequest(request Request) {
	conn, err := net.Dial("tcp", "localhost:9090")
	if err != nil {
		fmt.Println("Error connecting to TCP server:", err)
		return
	}
	defer conn.Close()

	data, err := json.Marshal(request)
	if err != nil {
		fmt.Println("Error encoding request:", err)
		return
	}

	_, err = conn.Write(data)
	if err != nil {
		fmt.Println("Error sending data:", err)
		return
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error membaca respons dari server:", err)
		return
	}

	fmt.Println("Response dari server:", string(buf[:n]))
}

func getAmount(reader *bufio.Reader) int {
	for {
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		amount, err := strconv.Atoi(input)
		if err != nil || amount < 0 {
			fmt.Println("Input tidak valid. Masukkan angka positif.")
			fmt.Print("Silakan masukkan lagi: ")
		} else {
			return amount
		}
	}
}
