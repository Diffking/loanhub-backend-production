package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// ============================================================
// Phase 4: SSE Hub + LINE Notify
// ============================================================

// SSEEvent represents a server-sent event
type SSEEvent struct {
	Event    string      `json:"event"`
	BranchID uint        `json:"branch_id"`
	Data     interface{} `json:"data"`
}

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID       string
	UserID   uint
	BranchID uint
	Channel  chan SSEEvent
	IsTV     bool // true = TV display client, false = user client
}

// SSEHub manages all SSE connections
type SSEHub struct {
	mu      sync.RWMutex
	clients map[string]*SSEClient
}

// NewSSEHub creates a new SSE hub
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[string]*SSEClient),
	}
}

// Register adds a new SSE client
func (h *SSEHub) Register(client *SSEClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client.ID] = client
	log.Printf("📡 SSE client registered: %s (user=%d, branch=%d, tv=%v) | total=%d",
		client.ID, client.UserID, client.BranchID, client.IsTV, len(h.clients))
}

// Unregister removes an SSE client
func (h *SSEHub) Unregister(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if client, ok := h.clients[clientID]; ok {
		close(client.Channel)
		delete(h.clients, clientID)
		log.Printf("📡 SSE client unregistered: %s | total=%d", clientID, len(h.clients))
	}
}

// BroadcastToBranch sends an event to all clients watching a specific branch
func (h *SSEHub) BroadcastToBranch(branchID uint, event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	event.BranchID = branchID
	sent := 0
	for _, client := range h.clients {
		if client.BranchID == branchID {
			select {
			case client.Channel <- event:
				sent++
			default:
				// Client channel full, skip
				log.Printf("⚠️ SSE channel full for client %s, skipping", client.ID)
			}
		}
	}
	if sent > 0 {
		log.Printf("📡 SSE broadcast [%s] to branch %d → %d clients", event.Event, branchID, sent)
	}
}

// SendToUser sends an event to a specific user
func (h *SSEHub) SendToUser(userID uint, event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		if client.UserID == userID && !client.IsTV {
			select {
			case client.Channel <- event:
				log.Printf("📡 SSE sent [%s] to user %d", event.Event, userID)
			default:
				log.Printf("⚠️ SSE channel full for user %d, skipping", userID)
			}
		}
	}
}

// BroadcastToTV sends an event only to TV display clients for a branch
func (h *SSEHub) BroadcastToTV(branchID uint, event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	event.BranchID = branchID
	for _, client := range h.clients {
		if client.BranchID == branchID && client.IsTV {
			select {
			case client.Channel <- event:
			default:
			}
		}
	}
}

// GetClientCount returns the number of connected clients
func (h *SSEHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ============================================================
// QueueNotifyService — orchestrates SSE + LINE Notify
// ============================================================

// QueueNotifyService handles real-time notifications
type QueueNotifyService struct {
	Hub            *SSEHub
	lineNotifyToken string
}

// NewQueueNotifyService creates a new notification service
func NewQueueNotifyService() *QueueNotifyService {
	token := os.Getenv("LINE_NOTIFY_TOKEN")
	if token == "" {
		log.Println("⚠️ LINE_NOTIFY_TOKEN not set — LINE notifications disabled")
	}
	return &QueueNotifyService{
		Hub:            NewSSEHub(),
		lineNotifyToken: token,
	}
}

// ============================================================
// Notification triggers (called from queue_service.go)
// ============================================================

// NotifyTicketCalled — เมื่อ OFFICER กด call-next / call
func (n *QueueNotifyService) NotifyTicketCalled(branchID uint, userID uint, ticketNumber string, counterName string) {
	data := map[string]interface{}{
		"ticket_number": ticketNumber,
		"counter_name":  counterName,
		"message":       fmt.Sprintf("คิว %s ถึงคิวแล้ว! กรุณาไปที่ %s", ticketNumber, counterName),
	}

	// SSE → user
	n.Hub.SendToUser(userID, SSEEvent{Event: "ticket_called", Data: data})

	// SSE → branch (all clients + TV)
	n.Hub.BroadcastToBranch(branchID, SSEEvent{Event: "queue_update", Data: data})

	// LINE Notify
	go n.sendLINENotify(fmt.Sprintf("\n🔔 คิว %s ถึงคิวแล้ว!\nกรุณาไปที่ %s", ticketNumber, counterName))
}

// NotifyNearlyTurn — เมื่อเหลืออีก ~N คิว
func (n *QueueNotifyService) NotifyNearlyTurn(userID uint, ticketNumber string, queueAhead int64) {
	data := map[string]interface{}{
		"ticket_number": ticketNumber,
		"queue_ahead":   queueAhead,
		"message":       fmt.Sprintf("คิว %s เหลืออีก %d คิว เตรียมตัวให้พร้อม!", ticketNumber, queueAhead),
	}

	// SSE → user only
	n.Hub.SendToUser(userID, SSEEvent{Event: "nearly_turn", Data: data})

	// LINE Notify
	go n.sendLINENotify(fmt.Sprintf("\n⏰ คิว %s เหลืออีก %d คิว\nเตรียมตัวให้พร้อม!", ticketNumber, queueAhead))
}

// NotifyQueueUpdate — general update for branch (serve, complete, skip, etc.)
func (n *QueueNotifyService) NotifyQueueUpdate(branchID uint, eventType string, data map[string]interface{}) {
	n.Hub.BroadcastToBranch(branchID, SSEEvent{Event: eventType, Data: data})
}

// NotifyBookingReminder — แจ้งเตือนล่วงหน้า 1 วัน
func (n *QueueNotifyService) NotifyBookingReminder(ticketNumber string, branchName string, slotDate string, slotTime string) {
	msg := fmt.Sprintf("\n📅 แจ้งเตือนนัดหมาย\nคิว: %s\nสาขา: %s\nวันที่: %s เวลา: %s\nกรุณามาตรงเวลา",
		ticketNumber, branchName, slotDate, slotTime)
	go n.sendLINENotify(msg)
}

// NotifyBookingCancelled — แจ้งเมื่อ booking ถูก auto-cancel
func (n *QueueNotifyService) NotifyBookingCancelled(userID uint, ticketNumber string) {
	data := map[string]interface{}{
		"ticket_number": ticketNumber,
		"message":       fmt.Sprintf("คิว %s ถูกยกเลิกอัตโนมัติ เนื่องจากไม่มา check-in ภายในเวลาที่กำหนด", ticketNumber),
	}
	n.Hub.SendToUser(userID, SSEEvent{Event: "booking_cancelled", Data: data})

	go n.sendLINENotify(fmt.Sprintf("\n❌ คิว %s ถูกยกเลิกอัตโนมัติ\nเนื่องจากไม่มา check-in ภายใน 30 นาที", ticketNumber))
}

// ============================================================
// LINE Notify HTTP sender
// ============================================================

func (n *QueueNotifyService) sendLINENotify(message string) {
	if n.lineNotifyToken == "" {
		log.Println("⚠️ LINE Notify skipped (no token)")
		return
	}

	payload := map[string]string{"message": message}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://notify-api.line.me/api/notify", bytes.NewReader(body))
	if err != nil {
		log.Printf("❌ LINE Notify request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+n.lineNotifyToken)

	// LINE Notify uses form data, not JSON
	formData := fmt.Sprintf("message=%s", message)
	req.Body = nil
	req, _ = http.NewRequest("POST", "https://notify-api.line.me/api/notify",
		bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+n.lineNotifyToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ LINE Notify send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Println("✅ LINE Notify sent successfully")
	} else {
		log.Printf("⚠️ LINE Notify status: %d", resp.StatusCode)
	}
}
