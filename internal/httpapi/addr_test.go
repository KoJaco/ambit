package httpapi

import "testing"

func TestValidateAddr(t *testing.T) {
	ok := []string{"127.0.0.1:8080", "127.0.0.1:0", "[::1]:8080"}
	for _, addr := range ok {
		if err := ValidateAddr(addr); err != nil {
			t.Errorf("%s: %v", addr, err)
		}
	}
	bad := []string{"0.0.0.0:8080", "192.168.1.1:8080", "localhost:8080", ":8080", "127.0.0.1"}
	for _, addr := range bad {
		if err := ValidateAddr(addr); err == nil {
			t.Errorf("%s: expected rejection", addr)
		}
	}
}
