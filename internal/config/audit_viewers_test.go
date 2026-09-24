package config

import "testing"

func TestCanViewAudit(t *testing.T) {
	c := &Config{AuditViewers: splitList(" 07337 , ,01234")}

	cases := map[string]bool{
		"07337": true,
		"01234": true,
		"7337":  false, // ต้องตรงทั้งเลข (มีเลข 0 นำหน้า)
		"":      false,
		"99999": false,
	}
	for membNo, want := range cases {
		if got := c.CanViewAudit(membNo); got != want {
			t.Errorf("CanViewAudit(%q) = %v, want %v", membNo, got, want)
		}
	}

	if (&Config{}).CanViewAudit("07337") {
		t.Error("empty allowlist must deny everyone")
	}
}
