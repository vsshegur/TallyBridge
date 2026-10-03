package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type FollowUp struct {
	ID            string    `json:"id"`
	Company       string    `json:"company"`
	Party         string    `json:"party"`
	Amount        float64   `json:"amount,omitempty"`
	PromiseDate   string    `json:"promiseDate,omitempty"`
	Note          string    `json:"note,omitempty"`
	Status        string    `json:"status"`
	LastContacted time.Time `json:"lastContacted,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type FollowUpList struct {
	Open         []FollowUp `json:"open"`
	DueToday     []FollowUp `json:"dueToday"`
	Overdue      []FollowUp `json:"overdue"`
	Completed    []FollowUp `json:"completed"`
	OpenCount    int        `json:"openCount"`
	DueCount     int        `json:"dueCount"`
	OverdueCount int        `json:"overdueCount"`
}

type followUpFile struct {
	Items []FollowUp `json:"items"`
}

var followUpMu sync.Mutex

func init() { backupNames["followups.json"] = true }

func (s *Server) followUpsPath() string { return filepath.Join(s.store.dir, "followups.json") }

func loadFollowUpsFile(path string) ([]FollowUp, error) {
	var f followUpFile
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []FollowUp{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return []FollowUp{}, nil
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, errors.New("follow-up file is damaged")
	}
	if f.Items == nil {
		f.Items = []FollowUp{}
	}
	return f.Items, nil
}

func saveFollowUpsFile(path string, items []FollowUp) error {
	b, err := json.MarshalIndent(followUpFile{Items: items}, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, b, 0600)
}

func cleanFollowUp(in FollowUp) (FollowUp, error) {
	in.Company = compactOrderText(in.Company, 160)
	in.Party = compactOrderText(in.Party, 160)
	in.Note = strings.TrimSpace(in.Note)
	if len(in.Note) > 1200 {
		return FollowUp{}, errors.New("follow-up note is too long")
	}
	if in.Company == "" || in.Party == "" {
		return FollowUp{}, errors.New("company and party are required")
	}
	if in.Amount < 0 || in.Amount > 1000000000000 {
		return FollowUp{}, errors.New("invalid follow-up amount")
	}
	in.PromiseDate = normISODate(strings.TrimSpace(in.PromiseDate))
	if in.PromiseDate != "" {
		if _, err := time.Parse("2006-01-02", in.PromiseDate); err != nil {
			return FollowUp{}, errors.New("promise date must be a valid date")
		}
	}
	if in.Status != "completed" {
		in.Status = "open"
	}
	return in, nil
}

func (s *Server) saveFollowUp(in FollowUp) (FollowUp, error) {
	clean, err := cleanFollowUp(in)
	if err != nil {
		return FollowUp{}, err
	}
	followUpMu.Lock()
	defer followUpMu.Unlock()
	items, err := loadFollowUpsFile(s.followUpsPath())
	if err != nil {
		return FollowUp{}, err
	}
	now := time.Now()
	if strings.TrimSpace(clean.ID) == "" {
		clean.ID = randomHex(12)
		clean.CreatedAt = now
		clean.UpdatedAt = now
		clean.LastContacted = now
		items = append(items, clean)
	} else {
		found := false
		for i := range items {
			if items[i].ID != clean.ID {
				continue
			}
			found = true
			clean.CreatedAt = items[i].CreatedAt
			clean.UpdatedAt = now
			if clean.LastContacted.IsZero() {
				clean.LastContacted = items[i].LastContacted
			}
			items[i] = clean
			break
		}
		if !found {
			return FollowUp{}, errors.New("follow-up not found")
		}
	}
	if err := saveFollowUpsFile(s.followUpsPath(), items); err != nil {
		return FollowUp{}, err
	}
	return clean, nil
}

func (s *Server) listFollowUps(company, party string) (FollowUpList, error) {
	followUpMu.Lock()
	defer followUpMu.Unlock()
	items, err := loadFollowUpsFile(s.followUpsPath())
	if err != nil {
		return FollowUpList{}, err
	}
	company = strings.TrimSpace(company)
	party = strings.TrimSpace(party)
	today := time.Now().Format("2006-01-02")
	out := FollowUpList{Open: []FollowUp{}, DueToday: []FollowUp{}, Overdue: []FollowUp{}, Completed: []FollowUp{}}
	for _, f := range items {
		if company != "" && !strings.EqualFold(company, strings.TrimSpace(f.Company)) {
			continue
		}
		if party != "" && !strings.EqualFold(party, strings.TrimSpace(f.Party)) {
			continue
		}
		if f.Status == "completed" {
			out.Completed = append(out.Completed, f)
			continue
		}
		out.Open = append(out.Open, f)
		if f.PromiseDate == today {
			out.DueToday = append(out.DueToday, f)
		} else if f.PromiseDate != "" && f.PromiseDate < today {
			out.Overdue = append(out.Overdue, f)
		}
	}
	sort.Slice(out.Open, func(i, j int) bool {
		a, b := out.Open[i].PromiseDate, out.Open[j].PromiseDate
		if a == "" && b != "" {
			return false
		}
		if a != "" && b == "" {
			return true
		}
		if a == b {
			return out.Open[i].UpdatedAt.After(out.Open[j].UpdatedAt)
		}
		return a < b
	})
	sort.Slice(out.Completed, func(i, j int) bool { return out.Completed[i].UpdatedAt.After(out.Completed[j].UpdatedAt) })
	if len(out.Completed) > 50 {
		out.Completed = out.Completed[:50]
	}
	out.OpenCount = len(out.Open)
	out.DueCount = len(out.DueToday)
	out.OverdueCount = len(out.Overdue)
	return out, nil
}

func (s *Server) handleFollowUps(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		x, err := s.listFollowUps(r.URL.Query().Get("company"), r.URL.Query().Get("party"))
		if err != nil {
			jsonErr(w, 500, err)
			return
		}
		jsonOut(w, x)
	case http.MethodPost:
		var in FollowUp
		if readJSON(r, &in) != nil {
			jsonErr(w, 400, errors.New("invalid follow-up"))
			return
		}
		x, err := s.saveFollowUp(in)
		if err != nil {
			jsonErr(w, 400, err)
			return
		}
		jsonOut(w, x)
	default:
		jsonErr(w, 405, errors.New("GET or POST required"))
	}
}
