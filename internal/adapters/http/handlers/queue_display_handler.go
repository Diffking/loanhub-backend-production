package handlers

import (
	"bufio"
	"fmt"
	"log"
	"strconv"
	"time"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// QueueDisplayHandler handles TV display endpoints (public, no auth)
type QueueDisplayHandler struct {
	queueService  *services.QueueService
	notifyService *services.QueueNotifyService
}

// NewQueueDisplayHandler creates a new display handler
func NewQueueDisplayHandler(queueService *services.QueueService, notifyService *services.QueueNotifyService) *QueueDisplayHandler {
	return &QueueDisplayHandler{
		queueService:  queueService,
		notifyService: notifyService,
	}
}

// ============================================================
// GET /api/v1/queue/display/:branch_id — ข้อมูลจอแสดง (Public)
// ============================================================
func (h *QueueDisplayHandler) GetDisplayData(c *fiber.Ctx) error {
	branchID, err := strconv.ParseUint(c.Params("branch_id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid branch ID")
	}

	data, err := h.queueService.GetDisplayData(uint(branchID))
	if err != nil {
		if err == services.ErrBranchNotFound {
			return response.NotFound(c, "Branch not found")
		}
		return response.InternalServerError(c, "Failed to get display data")
	}
	return response.Success(c, "Display data retrieved", data)
}

// ============================================================
// GET /api/v1/queue/display/:branch_id/events — SSE สำหรับจอ TV (Public)
// ============================================================
func (h *QueueDisplayHandler) DisplaySSE(c *fiber.Ctx) error {
	branchID, err := strconv.ParseUint(c.Params("branch_id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid branch ID")
	}

	// Verify branch exists
	_, err = h.queueService.GetBranchByID(uint(branchID))
	if err != nil {
		return response.NotFound(c, "Branch not found")
	}

	clientID := fmt.Sprintf("tv-%d-%d", branchID, time.Now().UnixNano())

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("Access-Control-Allow-Origin", "*")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		client := &services.SSEClient{
			ID:       clientID,
			UserID:   0,
			BranchID: uint(branchID),
			Channel:  make(chan services.SSEEvent, 50),
			IsTV:     true,
		}

		h.notifyService.Hub.Register(client)
		defer h.notifyService.Hub.Unregister(clientID)

		// Send initial connection event
		fmt.Fprintf(w, "event: connected\ndata: {\"client_id\":\"%s\",\"branch_id\":%d}\n\n", clientID, branchID)
		w.Flush()

		// Heartbeat ticker
		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case event, ok := <-client.Channel:
				if !ok {
					return
				}
				writeSSEEvent(w, event)
				w.Flush()

			case <-heartbeat.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				if err := w.Flush(); err != nil {
					log.Printf("📡 TV SSE client disconnected: %s", clientID)
					return
				}
			}
		}
	})

	return nil
}

// writeSSEEvent writes a formatted SSE event to the writer
func writeSSEEvent(w *bufio.Writer, event services.SSEEvent) {
	fmt.Fprintf(w, "event: %s\n", event.Event)

	// Simple JSON marshal inline
	switch data := event.Data.(type) {
	case map[string]interface{}:
		// Build JSON manually for simple maps
		jsonStr := "{"
		first := true
		for k, v := range data {
			if !first {
				jsonStr += ","
			}
			switch val := v.(type) {
			case string:
				jsonStr += fmt.Sprintf(`"%s":"%s"`, k, val)
			case int:
				jsonStr += fmt.Sprintf(`"%s":%d`, k, val)
			case int64:
				jsonStr += fmt.Sprintf(`"%s":%d`, k, val)
			case uint:
				jsonStr += fmt.Sprintf(`"%s":%d`, k, val)
			case float64:
				jsonStr += fmt.Sprintf(`"%s":%f`, k, val)
			case bool:
				jsonStr += fmt.Sprintf(`"%s":%v`, k, val)
			default:
				jsonStr += fmt.Sprintf(`"%s":"%v"`, k, val)
			}
			first = false
		}
		jsonStr += "}"
		fmt.Fprintf(w, "data: %s\n\n", jsonStr)
	default:
		fmt.Fprintf(w, "data: %v\n\n", data)
	}
}

// ============================================================
// Fiber adapter: convert fasthttp stream writer
// ============================================================

// Note: Fiber's c.Context().SetBodyStreamWriter() expects func(w *bufio.Writer)
// but the actual signature is fasthttp.StreamWriter = func(w *bufio.Writer)
// The above implementation works with Fiber v2 directly.

// For reference, if using raw fasthttp:
var _ fasthttp.StreamWriter = func(w *bufio.Writer) {}
