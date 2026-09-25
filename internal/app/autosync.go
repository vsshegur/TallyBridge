package app

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Server) autoSyncSnapshot() AutoSyncState { s.mu.Lock(); defer s.mu.Unlock(); return s.auto }
func (s *Server) setAuto(fn func(*AutoSyncState)) { s.mu.Lock(); fn(&s.auto); s.mu.Unlock() }
func (s *Server) startAutoSync(company, reason string) bool {
	s.mu.Lock()
	if s.auto.Running {
		s.mu.Unlock()
		return false
	}
	s.auto.Running = true
	s.auto.Company = company
	s.auto.Step = "Starting"
	s.auto.StartedAt = time.Now()
	s.auto.LastError = ""
	s.mu.Unlock()
	go s.runAutoSync(company, reason)
	return true
}
func (s *Server) runAutoSync(company, reason string) {
	defer s.setAuto(func(a *AutoSyncState) { a.Running = false; a.Step = "" })
	companies := []string{}
	if strings.TrimSpace(company) != "" {
		companies = []string{company}
	} else {
		// Probe is intentionally tiny. If it fails, keep saved data and retry later.
		if st, e := s.probeLive(); e == nil {
			for _, c := range st.Companies {
				if c.Name != "" {
					companies = append(companies, c.Name)
				}
			}
		}
		if len(companies) == 0 {
			if cs, _ := s.cache.Companies(); len(cs) > 0 {
				for _, c := range cs {
					companies = append(companies, c.Name)
				}
			}
		}
	}
	if len(companies) == 0 {
		s.setAuto(func(a *AutoSyncState) { a.LastError = "No open Tally company detected yet" })
		return
	}
	for _, c := range companies {
		s.setAuto(func(a *AutoSyncState) { a.Company = c })
		steps := []struct {
			name string
			fn   func() error
		}{
			{"Ledger index", func() error {
				x, e := s.getLedgers(c, true)
				if e != nil {
					return e
				}
				if !x.Live {
					return errors.New(x.LiveError)
				}
				return nil
			}},
			{"Receivable", func() error {
				x, e := s.getOutstanding(c, "receivable", true)
				if e != nil {
					return e
				}
				if !x.Live {
					return errors.New(x.LiveError)
				}
				return nil
			}},
			{"Sales this month", func() error {
				x, e := s.getVouchers(c, "sales", "month", true)
				if e != nil {
					return e
				}
				if !x.Live {
					return errors.New(x.LiveError)
				}
				return nil
			}},
			{"Payable", func() error {
				x, e := s.getOutstanding(c, "payable", true)
				if e != nil {
					return e
				}
				if !x.Live {
					return errors.New(x.LiveError)
				}
				return nil
			}},
		}
		for _, st := range steps {
			s.setAuto(func(a *AutoSyncState) { a.Step = st.name })
			if e := st.fn(); e != nil {
				s.setAuto(func(a *AutoSyncState) { a.LastError = fmt.Sprintf("%s: %v", st.name, e) })
				return
			}
		}
	}
	s.setAuto(func(a *AutoSyncState) { a.LastOK = time.Now(); a.LastError = "" })
}
func (s *Server) StartBackground() {
	go func() {
		time.Sleep(4 * time.Second)
		s.startAutoSync("", "startup")
		t := time.NewTicker(3 * time.Minute)
		defer t.Stop()
		for range t.C {
			s.startAutoSync("", "periodic")
		}
	}()
}
