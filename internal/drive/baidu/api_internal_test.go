package baidu

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoReqCookieInjection(t *testing.T) {
	// Create a mock server to check received cookies
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for BDUSS and SToken in cookies
		bdussFound := false
		stokenFound := false
		for _, cookie := range r.Cookies() {
			if cookie.Name == "BDUSS" && cookie.Value == "test-bduss" {
				bdussFound = true
			}
			if cookie.Name == "SToken" && cookie.Value == "test-stoken" {
				stokenFound = true
			}
		}
		
		if !bdussFound {
			t.Errorf("BDUSS cookie not found or invalid")
		}
		if !stokenFound {
			t.Errorf("SToken cookie not found or invalid")
		}
		
		// Check User-Agent
		ua := r.Header.Get("User-Agent")
		if ua == "" || ua == "Go-http-client/1.1" {
			t.Errorf("User-Agent spoofing failed, got: %s", ua)
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"errno": 0}`))
	}))
	defer server.Close()
	
	// Initialize plugin with test tokens
	plugin := NewBaiduPlugin("test-bduss", "test-stoken")
	
	// Test doReq
	resp, err := plugin.doReq(context.Background(), "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("doReq failed: %v", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"errno": 0}` {
		t.Errorf("Unexpected response data: %s", string(body))
	}
}
