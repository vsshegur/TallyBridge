package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxOrderPhotos      = 5
	maxOrderPhotoSize   = 3 * 1024 * 1024
	maxPhotoJSONBody    = 6 * 1024 * 1024
	maxOrderPhotoSide   = 12000
	maxOrderPhotoPixels = 40_000_000
)

type OrderAttachment struct {
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	ContentType string    `json:"contentType"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"createdAt"`
}

type orderPhotoUpload struct {
	OrderID     string `json:"orderId"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	SHA256      string `json:"sha256"`
	Data        string `json:"data"`
}

func (s *Server) orderImagesDir() string { return filepath.Join(s.store.dir, "order-images") }

func orderPhotoExt(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return ".png"
	default:
		return ".jpg"
	}
}

func (s *Server) orderAttachmentPath(a OrderAttachment) string {
	return filepath.Join(s.orderImagesDir(), a.ID+orderPhotoExt(a.ContentType))
}

func (s *Server) deleteOrderImages(o SalesOrder) {
	for _, a := range o.Attachments {
		if strings.TrimSpace(a.ID) == "" {
			continue
		}
		_ = os.Remove(s.orderAttachmentPath(a))
	}
}

func validatePhotoBytes(contentType string, data []byte) error {
	if len(data) == 0 || len(data) > maxOrderPhotoSize {
		return fmt.Errorf("order photo must be between 1 byte and %d MB", maxOrderPhotoSize/(1024*1024))
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct != "image/jpeg" && ct != "image/png" {
		return errors.New("only JPEG or PNG order photos are supported")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return errors.New("order photo is not a valid image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxOrderPhotoSide || cfg.Height > maxOrderPhotoSide || int64(cfg.Width)*int64(cfg.Height) > maxOrderPhotoPixels {
		return errors.New("order photo dimensions are too large")
	}
	if (format == "jpeg" && ct != "image/jpeg") || (format == "png" && ct != "image/png") {
		return errors.New("order photo content type does not match the image")
	}
	return nil
}

func cleanPhotoName(name string) string {
	name = compactOrderText(name, 120)
	name = filepath.Base(name)
	if name == "." || name == "" {
		return "order-photo.jpg"
	}
	return name
}

func (s *Server) saveOrderPhoto(q orderPhotoUpload) (OrderAttachment, error) {
	q.OrderID = strings.TrimSpace(q.OrderID)
	q.ContentType = strings.ToLower(strings.TrimSpace(q.ContentType))
	if q.OrderID == "" {
		return OrderAttachment{}, errors.New("order id required")
	}
	if len(q.Data) > maxPhotoJSONBody {
		return OrderAttachment{}, errors.New("order photo payload is too large")
	}
	data, err := base64.StdEncoding.DecodeString(q.Data)
	if err != nil {
		return OrderAttachment{}, errors.New("invalid order photo data")
	}
	if err := validatePhotoBytes(q.ContentType, data); err != nil {
		return OrderAttachment{}, err
	}
	sum := sha256.Sum256(data)
	actualSHA := hex.EncodeToString(sum[:])
	if q.SHA256 != "" && !strings.EqualFold(strings.TrimSpace(q.SHA256), actualSHA) {
		return OrderAttachment{}, errors.New("order photo checksum mismatch")
	}

	orderMu.Lock()
	defer orderMu.Unlock()
	orders, err := s.loadOrdersLocked(time.Now())
	if err != nil {
		return OrderAttachment{}, err
	}
	idx := -1
	for i := range orders {
		if orders[i].ID == q.OrderID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return OrderAttachment{}, errors.New("order not found")
	}
	if orders[idx].Status == "dispatched" {
		return OrderAttachment{}, errors.New("cannot add photos to a dispatched order")
	}
	for _, existing := range orders[idx].Attachments {
		if strings.EqualFold(existing.SHA256, actualSHA) {
			return existing, nil
		}
	}
	if len(orders[idx].Attachments) >= maxOrderPhotos {
		return OrderAttachment{}, fmt.Errorf("an order can have at most %d photos", maxOrderPhotos)
	}
	if err := os.MkdirAll(s.orderImagesDir(), 0700); err != nil {
		return OrderAttachment{}, err
	}
	att := OrderAttachment{
		ID:          randomHex(12),
		Name:        cleanPhotoName(q.Name),
		ContentType: q.ContentType,
		Size:        int64(len(data)),
		SHA256:      actualSHA,
		CreatedAt:   time.Now(),
	}
	path := s.orderAttachmentPath(att)
	if err := atomicWrite(path, data, 0600); err != nil {
		return OrderAttachment{}, err
	}
	orders[idx].PhotoOrder = true
	orders[idx].Attachments = append(orders[idx].Attachments, att)
	orders[idx].UpdatedAt = time.Now()
	if err := s.saveOrdersLocked(orders); err != nil {
		_ = os.Remove(path)
		return OrderAttachment{}, err
	}
	return att, nil
}

func (s *Server) serveOrderPhoto(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.URL.Query().Get("order"))
	attachmentID := strings.TrimSpace(r.URL.Query().Get("attachment"))
	if orderID == "" || attachmentID == "" {
		jsonErr(w, 400, errors.New("order and attachment are required"))
		return
	}
	orderMu.Lock()
	orders, err := s.loadOrdersLocked(time.Now())
	if err != nil {
		orderMu.Unlock()
		jsonErr(w, 500, err)
		return
	}
	var att *OrderAttachment
	for i := range orders {
		if orders[i].ID != orderID {
			continue
		}
		for j := range orders[i].Attachments {
			if orders[i].Attachments[j].ID == attachmentID {
				copy := orders[i].Attachments[j]
				att = &copy
				break
			}
		}
		break
	}
	orderMu.Unlock()
	if att == nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(s.orderAttachmentPath(*att))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		jsonErr(w, 500, err)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		jsonErr(w, 500, err)
		return
	}
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Disposition", `inline; filename="`+strings.ReplaceAll(att.Name, `"`, "")+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	http.ServeContent(w, r, att.Name, st.ModTime(), f)
}

func (s *Server) handleOrderPhotoUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonErr(w, 405, errors.New("POST required"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoJSONBody)
	defer r.Body.Close()
	var q orderPhotoUpload
	dec := json.NewDecoder(io.LimitReader(r.Body, maxPhotoJSONBody))
	if err := dec.Decode(&q); err != nil {
		jsonErr(w, 400, errors.New("invalid order photo payload"))
		return
	}
	att, err := s.saveOrderPhoto(q)
	if err != nil {
		jsonErr(w, 400, err)
		return
	}
	jsonOut(w, att)
}
