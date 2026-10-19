package app

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func utf16le(s string, bom bool) []byte {
	var b bytes.Buffer
	if bom {
		b.Write([]byte{0xFF, 0xFE})
	}
	for _, u := range utf16.Encode([]rune(s)) {
		_ = binary.Write(&b, binary.LittleEndian, u)
	}
	return b.Bytes()
}

func TestTallyToUTF8(t *testing.T) {
	doc := `<?xml version="1.0" encoding="UTF-16"?><ENVELOPE><NAME>श्री Traders ₹</NAME></ENVELOPE>`
	for _, bom := range []bool{true, false} {
		got := string(tallyToUTF8(utf16le(doc, bom)))
		if !strings.Contains(got, `encoding="UTF-8"`) || !strings.Contains(got, "श्री Traders ₹") {
			t.Fatalf("bom=%v: %q", bom, got)
		}
		clean, n := sanitizeXML([]byte(got))
		if n != 0 || !bytes.Contains(clean, []byte("₹")) {
			t.Fatalf("sanitize damaged converted text: %d %q", n, clean)
		}
	}
	plain := []byte("<ENVELOPE>A</ENVELOPE>")
	if !bytes.Equal(tallyToUTF8(plain), plain) {
		t.Fatal("plain UTF-8 changed")
	}
	if got := tallyToUTF8(append([]byte{0xEF, 0xBB, 0xBF}, plain...)); !bytes.Equal(got, plain) {
		t.Fatal("UTF-8 BOM kept", got)
	}
}

func TestTallyUTF16ReplyIsUsable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-16")
		w.Write(utf16le(`<?xml version="1.0" encoding="UTF-16"?><ENVELOPE><BODY><DATA><COLLECTION><COMPANY NAME="Shree Ganesh ₹ Traders"><NAME>Shree Ganesh ₹ Traders</NAME></COMPANY></COLLECTION></DATA></BODY></ENVELOPE>`, true))
	}))
	defer ts.Close()
	s, e := NewServer(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	cfg := s.store.Settings()
	cfg.TallyURL = ts.URL
	if e = s.store.UpdateSettings(cfg); e != nil {
		t.Fatal(e)
	}
	out, _, _, e := s.tally.request("companies", "", []byte(`<ENVELOPE><HEADER><TALLYREQUEST>Export</TALLYREQUEST></HEADER><BODY><DESC><STATICVARIABLES><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES></DESC></BODY></ENVELOPE>`), 0)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out, []byte("Shree Ganesh ₹ Traders")) {
		t.Fatalf("company name lost: %q", out)
	}
}

func TestAtomicWriteReplacesFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if e := atomicWrite(p, []byte("one"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := atomicWrite(p, []byte("two"), 0600); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "two" {
		t.Fatal(string(b))
	}
	if _, e := os.Stat(p + ".tmp"); !os.IsNotExist(e) {
		t.Fatal("temp file left behind")
	}
}

// A phone photo of ~2.5 MB becomes ~3.4 MB of base64; the signed request must
// get past phone authentication and reach the photo handler's own 6 MB limit.
func TestLargeSignedPhotoPassesAuth(t *testing.T) {
	s, key := nativeFixture(t)
	token := accessToken(t, s, key, nil)
	phone, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub, _ := x509.MarshalPKIXPublicKey(&phone.PublicKey)
	code, _ := s.store.GeneratePairCode()
	body, _ := json.Marshal(map[string]string{"code": code, "name": "Phone", "publicKey": base64.StdEncoding.EncodeToString(pub)})
	h := s.NativeHandlerWithOrders()
	w := serve(h, nativeRequest(t, "POST", "/api/v2/native/pair", string(body), token, "", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var paired map[string]any
	json.Unmarshal(w.Body.Bytes(), &paired)
	id := paired["deviceId"].(string)
	s.native.mu.Lock()
	for i := range s.native.Devices {
		s.native.Devices[i].Approved = true
	}
	s.native.mu.Unlock()

	big := `{"data":"` + strings.Repeat("A", 3_400_000) + `"}`
	w = serve(h, nativeRequest(t, "POST", "/api/v1/mobile/orders/photo", big, token, id, phone))
	if strings.Contains(w.Body.String(), "too large") || w.Code == 403 || w.Code == 401 {
		t.Fatal("photo rejected before the photo handler:", w.Code, w.Body.String())
	}
	w = serve(h, nativeRequest(t, "POST", "/api/v1/mobile/orders", big, token, id, phone))
	if w.Code != 403 || !strings.Contains(w.Body.String(), "too large") {
		t.Fatal("non-photo route accepted a 3 MB body", w.Code, w.Body.String())
	}
}
