package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
)

// LINEConfig holds LINE API configuration
type LINEConfig struct {
	ChannelID     string
	LIFFChannelID string // ✅ LIFF Channel ID (อาจต่างจาก LINE Login Channel)
	ChannelSecret string
	CallbackURL   string
}

// LINEService handles LINE Login and Messaging
type LINEService struct {
	db     *gorm.DB
	config LINEConfig
}

// LINETokenResponse represents LINE token response
type LINETokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token"`
}

// LINEProfile represents LINE user profile
type LINEProfile struct {
	UserID        string `json:"userId"`
	DisplayName   string `json:"displayName"`
	PictureURL    string `json:"pictureUrl"`
	StatusMessage string `json:"statusMessage"`
}

// LINETokenVerifyResponse represents LINE token verify response
// ============================================================
// ✅ เพิ่มใหม่: สำหรับ verify LINE access token
// ============================================================
type LINETokenVerifyResponse struct {
	Scope     string `json:"scope"`
	ClientID  string `json:"client_id"`
	ExpiresIn int    `json:"expires_in"`
}

// NewLINEService creates a new LINE service
func NewLINEService(db *gorm.DB, channelID, channelSecret, callbackURL, liffChannelID string) *LINEService {
	if liffChannelID == "" {
		liffChannelID = channelID // fallback ใช้ตัวเดียวกัน
	}
	return &LINEService{
		db: db,
		config: LINEConfig{
			ChannelID:     channelID,
			LIFFChannelID: liffChannelID,
			ChannelSecret: channelSecret,
			CallbackURL:   callbackURL,
		},
	}
}

// GetLoginURL generates LINE Login URL
func (s *LINEService) GetLoginURL(state string) string {
	params := url.Values{}
	params.Add("response_type", "code")
	params.Add("client_id", s.config.ChannelID)
	params.Add("redirect_uri", s.config.CallbackURL)
	params.Add("state", state)
	params.Add("scope", "profile openid")

	return fmt.Sprintf("https://access.line.me/oauth2/v2.1/authorize?%s", params.Encode())
}

// ExchangeToken exchanges authorization code for access token
func (s *LINEService) ExchangeToken(code string) (*LINETokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", s.config.CallbackURL)
	data.Set("client_id", s.config.ChannelID)
	data.Set("client_secret", s.config.ChannelSecret)

	req, err := http.NewRequest("POST", "https://api.line.me/oauth2/v2.1/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LINE token error: %s", string(body))
	}

	var tokenResp LINETokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// GetProfile gets LINE user profile
func (s *LINEService) GetProfile(accessToken string) (*LINEProfile, error) {
	req, err := http.NewRequest("GET", "https://api.line.me/v2/profile", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LINE profile error: %s", string(body))
	}

	var profile LINEProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// ============================================================
// ✅ เพิ่มใหม่: VerifyAccessToken - ตรวจสอบ LINE Access Token
// เรียก LINE API เพื่อ verify ว่า token นี้ถูกต้อง
// และมาจาก Channel ของเราจริงๆ
// ============================================================
func (s *LINEService) VerifyAccessToken(accessToken string) (*LINETokenVerifyResponse, error) {
	// เรียก LINE Verify API
	verifyURL := fmt.Sprintf("https://api.line.me/oauth2/v2.1/verify?access_token=%s", url.QueryEscape(accessToken))

	req, err := http.NewRequest("GET", verifyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create verify request failed: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LINE verify request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read verify response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LINE token invalid: %s", string(body))
	}

	var verifyResp LINETokenVerifyResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		return nil, fmt.Errorf("parse verify response failed: %w", err)
	}

	// ✅ ตรวจสอบว่า token มาจาก Channel ของเราจริง
	if verifyResp.ClientID != s.config.ChannelID && verifyResp.ClientID != s.config.LIFFChannelID {
		return nil, fmt.Errorf("LINE token channel_id mismatch: expected %s or %s, got %s",
			s.config.ChannelID, s.config.LIFFChannelID, verifyResp.ClientID)
	}

	// ตรวจว่า token ยังไม่หมดอายุ
	if verifyResp.ExpiresIn <= 0 {
		return nil, fmt.Errorf("LINE token expired")
	}

	return &verifyResp, nil
}

// ============================================================
// ✅ เพิ่มใหม่: VerifyAndGetProfile - Verify token แล้วดึง profile
// รวม 2 ขั้นตอนเป็น 1 function เพื่อความสะดวก
// ============================================================
func (s *LINEService) VerifyAndGetProfile(accessToken string) (*LINEProfile, error) {
	// Step 1: Verify token
	_, err := s.VerifyAccessToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	// Step 2: Get profile
	profile, err := s.GetProfile(accessToken)
	if err != nil {
		return nil, fmt.Errorf("get profile failed: %w", err)
	}

	return profile, nil
}

// LinkUserLINE links LINE account to user
func (s *LINEService) LinkUserLINE(userID uint, lineUserID, displayName string) error {
	now := time.Now()
	result := s.db.Exec(`
		UPDATE users 
		SET line_user_id = ?, line_display_name = ?, line_linked_at = ?
		WHERE id = ?
	`, lineUserID, displayName, now, userID)

	if result.Error != nil {
		return result.Error
	}
	return nil
}

// UnlinkUserLINE unlinks LINE account from user
func (s *LINEService) UnlinkUserLINE(userID uint) error {
	result := s.db.Exec(`
		UPDATE users 
		SET line_user_id = NULL, line_display_name = NULL, line_linked_at = NULL
		WHERE id = ?
	`, userID)

	if result.Error != nil {
		return result.Error
	}
	return nil
}

// GetUserByLINEID gets user by LINE User ID
func (s *LINEService) GetUserByLINEID(lineUserID string) (uint, error) {
	var userID uint
	result := s.db.Raw(`SELECT id FROM users WHERE line_user_id = ?`, lineUserID).Scan(&userID)
	if result.Error != nil {
		return 0, result.Error
	}
	return userID, nil
}

// SendPushMessage sends push message to LINE user
func (s *LINEService) SendPushMessage(lineUserID, message string, channelAccessToken string) error {
	payload := map[string]interface{}{
		"to": lineUserID,
		"messages": []map[string]interface{}{
			{
				"type": "text",
				"text": message,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.line.me/v2/bot/message/push", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+channelAccessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LINE push error: %s", string(body))
	}

	return nil
}

// SendFlexMessage sends flex message to LINE user
func (s *LINEService) SendFlexMessage(lineUserID string, flexContent map[string]interface{}, channelAccessToken string) error {
	payload := map[string]interface{}{
		"to": lineUserID,
		"messages": []map[string]interface{}{
			{
				"type":     "flex",
				"altText":  "แจ้งเตือนนัดหมาย",
				"contents": flexContent,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.line.me/v2/bot/message/push", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+channelAccessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LINE flex error: %s", string(body))
	}

	return nil
}

// CreateAppointmentReminder creates flex message for appointment reminder
func (s *LINEService) CreateAppointmentReminder(memberName, apptDate, apptTime, location, webURL string) map[string]interface{} {
	return map[string]interface{}{
		"type": "bubble",
		"header": map[string]interface{}{
			"type":            "box",
			"layout":          "vertical",
			"backgroundColor": "#1DB446",
			"paddingAll":      "15px",
			"contents": []map[string]interface{}{
				{
					"type":   "text",
					"text":   "📅 แจ้งเตือนนัดหมาย",
					"color":  "#FFFFFF",
					"weight": "bold",
					"size":   "lg",
				},
			},
		},
		"body": map[string]interface{}{
			"type":   "box",
			"layout": "vertical",
			"contents": []map[string]interface{}{
				{
					"type":   "text",
					"text":   fmt.Sprintf("สวัสดีคุณ %s", memberName),
					"weight": "bold",
					"size":   "md",
					"margin": "md",
				},
				{
					"type":   "text",
					"text":   "คุณมีนัดหมายกับสหกรณ์",
					"size":   "sm",
					"color":  "#666666",
					"margin": "sm",
				},
				{
					"type":   "separator",
					"margin": "lg",
				},
				{
					"type":   "box",
					"layout": "vertical",
					"margin": "lg",
					"contents": []map[string]interface{}{
						{
							"type":   "box",
							"layout": "horizontal",
							"contents": []map[string]interface{}{
								{"type": "text", "text": "📆 วันที่", "size": "sm", "color": "#555555", "flex": 0},
								{"type": "text", "text": apptDate, "size": "sm", "color": "#111111", "align": "end"},
							},
						},
						{
							"type":   "box",
							"layout": "horizontal",
							"margin": "sm",
							"contents": []map[string]interface{}{
								{"type": "text", "text": "⏰ เวลา", "size": "sm", "color": "#555555", "flex": 0},
								{"type": "text", "text": apptTime, "size": "sm", "color": "#111111", "align": "end"},
							},
						},
					},
				},
			},
		},
		"footer": map[string]interface{}{
			"type":   "box",
			"layout": "vertical",
			"contents": []map[string]interface{}{
				{
					"type":   "button",
					"style":  "primary",
					"color":  "#1DB446",
					"action": map[string]interface{}{
						"type":  "uri",
						"label": "🔗 เข้าสู่ระบบ",
						"uri":   webURL,
					},
				},
			},
		},
	}
}
